package commands

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/kardianos/osext"

	"code.cloudfoundry.org/lager"

	"cred-alert/inflator"
	"cred-alert/kolsch"
	"cred-alert/mimetype"
	"cred-alert/scanners"
	"cred-alert/scanners/diffscanner"
	"cred-alert/scanners/dirscanner"
	"cred-alert/scanners/filescanner"
	"cred-alert/sniff"
	"cred-alert/sniff/matchers"
)

type ScanCommand struct {
	File            string `short:"f" long:"file" description:"the file or directory to scan" value-name:"FILE"`
	Diff            bool   `long:"diff" description:"content to be scanned is a git diff"`
	ShowCredentials bool   `long:"show-suspected-credentials" description:"allow credentials to be shown in output"`
	Regexp          string `long:"regexp" description:"override default regexp matcher" value-name:"REGEXP"`
	RegexpFile      string `long:"regexp-file" description:"path to regexp file" value-name:"PATH"`
}

func (command *ScanCommand) Execute(args []string) error {
	warnIfOldExecutable()

	logger := lager.NewLogger("scan")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.INFO))

	var sniffer sniff.Sniffer
	if command.Regexp != "" && command.RegexpFile != "" {
		fmt.Fprintln(os.Stderr, yellow("[WARN]"), "Two options specified for Regexp, only using: --regexp", command.Regexp)
		sniffer = createSniffer(command.Regexp, "")
	} else {
		sniffer = createSniffer(command.Regexp, command.RegexpFile)
	}

	inflate := inflator.New()

	exitFuncs := []func(){
		func() { inflate.Close() },
	}

	signalsCh := make(chan os.Signal, 1)
	signal.Notify(signalsCh, os.Interrupt)

	go func() {
		for range signalsCh {
			for _, f := range exitFuncs {
				f()
			}
			os.Exit(1)
		}
	}()

	credsFound := 0

	if command.File != "" {
		fi, err := os.Stat(command.File)
		if err != nil {
			log.Fatalln(err.Error())
		}

		if fi.IsDir() {
			credsFound = scanDirectory(sniffer, command.File, command.ShowCredentials)
		} else {
			fh, err := os.Open(command.File)
			if err != nil {
				log.Fatalln(err.Error())
			}

			br := bufio.NewReader(fh)
			if mime, isArchive := mimetype.IsArchive(logger, br); isArchive {
				credsFound = scanArchive(logger, sniffer, mime, inflate, command.File, command.ShowCredentials)
			} else {
				credsFound = scanFile(logger, sniffer, br, command.File, command.ShowCredentials)
			}
		}
	} else if command.Diff {
		credsFound = scanDiff(logger, sniffer, command.ShowCredentials)
	} else {
		credsFound = scanFile(logger, sniffer, os.Stdin, "STDIN", command.ShowCredentials)
	}

	if credsFound > 0 {
		fmt.Println()
		fmt.Println("Yikes! Looks like we found some credentials.")
		fmt.Println()
		fmt.Println("There are a few cases for what this may be:")
		fmt.Println()
		fmt.Println("1. An actual credential in a repository which shouldn't be")
		fmt.Println("   committed! Remove it and try committing again.")
		fmt.Println()
		fmt.Println("2. An example credential in tests or documentation. You can")
		fmt.Println("   use the words 'fake' and/or 'example' in your credential so it is")
		fmt.Println("   ignored.")
		fmt.Println()
		fmt.Println("3. An actual credential in a credential repository. If you are calling this")
		fmt.Println("   via Git hook and if you want the false positive to go away, you can pass `-n`")
		fmt.Println("   to skip the hook for now.")
		fmt.Println()
		fmt.Println("4. A false positive which isn't a credential at all! Please let us know about ")
		fmt.Println("   the this case in our Slack channel (#pcf-sec-enablement).")

		exitFuncs = append(exitFuncs, func() {
			os.Exit(3)
		})
	}

	for _, f := range exitFuncs {
		f()
	}

	return nil
}

func scanArchive(
	logger lager.Logger,
	sniffer sniff.Sniffer,
	mime string,
	inflate inflator.Inflator,
	file string,
	showCredentials bool,
) int {
	inflateDir, err := ioutil.TempDir("", "cred-alert-cli")
	if err != nil {
		log.Fatalln(err.Error())
	}
	defer os.RemoveAll(inflateDir)

	inflateStart := time.Now()
	fmt.Print("Inflating archive... ")
	err = inflate.Inflate(logger, mime, file, inflateDir)
	if err != nil {
		fmt.Printf("%s\n", red("FAILED"))
		log.Fatalln(err.Error())
	}
	fmt.Printf("%s\n", green("DONE"))

	inflateDuration := time.Since(inflateStart)

	fmt.Println()
	fmt.Println("Time taken (inflating):", inflateDuration)
	fmt.Println("Any archive inflation errors can be found in: ", inflate.LogPath())
	fmt.Println()

	return scanDirectory(sniffer, inflateDir, showCredentials)
}

func scanDirectory(
	sniffer sniff.Sniffer,
	scanDir string,
	showCredentials bool,
) int {
	scanStart := time.Now()
	credsFound := 0
	violationsDir, err := ioutil.TempDir("", "cred-alert-cli-violations")
	if err != nil {
		log.Fatalln(err.Error())
	}
	archiveViolationHandler := func(logger lager.Logger, violation scanners.Violation) error {
		line := violation.Line
		credsFound++

		relPath, err := filepath.Rel(scanDir, line.Path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(violationsDir, relPath)
		err = os.MkdirAll(filepath.Dir(destPath), os.ModePerm)
		if err != nil {
			return err
		}

		err = persistFile(line.Path, destPath)
		if err != nil {
			return err
		}
		if showCredentials {
			fmt.Printf("%s %s:%d [%s]\n", red("[CRED]"), destPath, line.LineNumber, violation.Credential())
		} else {
			fmt.Printf("%s %s:%d\n", red("[CRED]"), destPath, line.LineNumber)
		}

		return nil
	}

	dirScanner := dirscanner.New(archiveViolationHandler, sniffer)
	err = dirScanner.Scan(kolsch.NewLogger(), scanDir)
	if err != nil {
		log.Fatalln(err.Error())
	}

	fmt.Println()
	fmt.Println("Scan complete!")
	fmt.Println()
	fmt.Println("Time taken (scanning):", time.Since(scanStart))
	fmt.Println("Credentials found:", credsFound)

	return credsFound
}

func createSniffer(regexp, regexpFile string) sniff.Sniffer {
	if regexp != "" {
		matcher := matchers.Format(regexp)
		exclusionMatcher := matchers.NewNullMatcher()

		return sniff.NewSniffer(matcher, exclusionMatcher)

	} else if regexpFile != "" {
		fh, err := os.Open(regexpFile)
		if err != nil {
			log.Fatalln(err.Error())
		}
		defer fh.Close()

		scanner := bufio.NewScanner(fh)
		var multi []matchers.Matcher
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			multi = append(multi, matchers.Format(string(bytes.ToUpper(line))))
		}

		matcher := matchers.UpcasedMulti(multi...)
		exclusionMatcher := matchers.NewNullMatcher()

		return sniff.NewSniffer(matcher, exclusionMatcher)
	}

	return sniff.NewDefaultSniffer()
}

func persistFile(srcPath, destPath string) error {
	destFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer destFile.Close()

	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	_, err = io.Copy(destFile, srcFile)
	return err
}

func scanFile(logger lager.Logger, sniffer sniff.Sniffer, f io.Reader, name string, showCredentials bool) int {
	scanner := filescanner.New(f, name)
	return countWithSniffer(logger, sniffer, showCredentials, scanner)
}

func scanDiff(logger lager.Logger, sniffer sniff.Sniffer, showCredentials bool) int {
	scanner := diffscanner.NewDiffScanner(os.Stdin)
	return countWithSniffer(logger, sniffer, showCredentials, scanner)
}

func countWithSniffer(logger lager.Logger, sniffer sniff.Sniffer, showCredentials bool, scanner sniff.Scanner) int {
	credsFound := 0

	handler := func(logger lager.Logger, violation scanners.Violation) error {
		credsFound++
		output := fmt.Sprintf("%s ", red("[CRED]"))
		if violation.Line.Path == ".git/COMMIT_EDITMSG" {
			output = output + "line "
		} else {
			output = output + fmt.Sprintf("%s:", violation.Line.Path)
		}
		output = output + fmt.Sprintf("%d", violation.Line.LineNumber)
		if showCredentials {
			output = output + fmt.Sprintf(" [%s]", violation.Credential())
		}
		fmt.Println(output)

		return nil
	}

	sniffer.Sniff(logger, scanner, handler)

	return credsFound
}

func warnIfOldExecutable() {
	const twoWeeks = 14 * 24 * time.Hour

	exePath, err := osext.Executable()
	if err != nil {
		return
	}

	info, err := os.Stat(exePath)
	if err != nil {
		return
	}

	mtime := info.ModTime()

	if time.Since(mtime) > twoWeeks {
		fmt.Fprintln(os.Stderr, yellow("[WARN]"), "Executable is old! Please consider running `cred-alert-cli update`.")
	}
}

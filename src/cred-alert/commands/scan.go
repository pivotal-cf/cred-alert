package commands

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
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

	if command.Regexp != "" && command.RegexpFile != "" {
		fmt.Fprintln(os.Stderr, yellow("[WARN]"), "Two options specified for Regexp, only using: --regexp", command.Regexp)
	}

	var sniffer sniff.Sniffer
	switch {
	case command.Regexp != "":
		sniffer = sniff.NewSniffer(matchers.Format(command.Regexp), nil)
	case command.RegexpFile != "":
		file, err := os.Open(command.RegexpFile)
		if err != nil {
			return err
		}
		defer file.Close()

		matcher := matchers.UpcasedMultiMatcherFromReader(file)
		sniffer = sniff.NewSniffer(matcher, nil)
	default:
		sniffer = sniff.NewDefaultSniffer()
	}

	signalsCh := make(chan os.Signal)
	signal.Notify(signalsCh, os.Interrupt)

	var exitFuncs []func()
	go func() {
		<-signalsCh
		log.SetFlags(0)
		log.Println("\ncleaning up...")
		for _, f := range exitFuncs {
			f()
		}
		os.Exit(1)
	}()

	var credsFound int
	handler := func(logger lager.Logger, violation scanners.Violation) error {
		line := violation.Line
		credsFound++
		output := fmt.Sprintf("%s %s:%d", red("[CRED]"), line.Path, line.LineNumber)
		if command.ShowCredentials {
			output = output + fmt.Sprintf(" [%s]", violation.Credential())
		}
		fmt.Println(output)

		return nil
	}

	quietLogger := kolsch.NewLogger()

	switch {
	case command.File != "":
		fi, err := os.Stat(command.File)
		if err != nil {
			return err
		}

		if fi.IsDir() {
			dirScanner := dirscanner.New(handler, sniffer)
			err = dirScanner.Scan(quietLogger, command.File)
			if err != nil {
				return err
			}
		} else {
			file, err := os.Open(command.File)
			if err != nil {
				return err
			}

			br := bufio.NewReader(file)
			if mime, isArchive := mimetype.IsArchive(logger, br); isArchive {
				inflateDir, err := ioutil.TempDir("", "cred-alert-cli")
				if err != nil {
					return err
				}

				inflate := inflator.New()
				exitFuncs = append(exitFuncs, func() {
					inflate.Close()
					os.RemoveAll(inflateDir)
				})

				inflateArchive(quietLogger, inflate, inflateDir, mime, command.File)

				violationsDir, err := ioutil.TempDir("", "cred-alert-cli-violations")
				if err != nil {
					return err
				}

				archiveHandler := sniff.NewArchiveViolationHandlerFunc(inflateDir, violationsDir, handler)
				scanner := dirscanner.New(archiveHandler, sniffer)

				err = scanner.Scan(quietLogger, inflateDir)
				if err != nil {
					return err
				}
			} else {
				sniffer.Sniff(logger, filescanner.New(br, command.File), handler)
			}
		}
	case command.Diff:
		sniffer.Sniff(logger, diffscanner.NewDiffScanner(os.Stdin), handler)
	default:
		sniffer.Sniff(logger, filescanner.New(os.Stdin, "STDIN"), handler)
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

func inflateArchive(
	logger lager.Logger,
	inflate inflator.Inflator,
	inflateDir string,
	mime string,
	file string,
) error {
	inflateStart := time.Now()
	fmt.Print("Inflating archive... ")
	err := inflate.Inflate(logger, mime, file, inflateDir)
	if err != nil {
		fmt.Printf("%s\n", red("FAILED"))
		return err
	}
	fmt.Printf("%s\n", green("DONE"))

	fmt.Println()
	fmt.Println("Time taken (inflating):", time.Since(inflateStart))
	fmt.Println("Any archive inflation errors can be found in: ", inflate.LogPath())
	fmt.Println()

	return nil
}

func copyFile(srcPath, destPath string) error {
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

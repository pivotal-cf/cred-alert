package commands

import (
	"bufio"
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
)

type ScanCommand struct {
	File            string `short:"f" long:"file" description:"the file to scan" value-name:"FILE"`
	Diff            bool   `long:"diff" description:"content to be scanned is a git diff"`
	ShowCredentials bool   `long:"show-suspected-credentials" description:"allow credentials to be shown in output"`
}

func (command *ScanCommand) Execute(args []string) error {
	warnIfOldExecutable()

	logger := kolsch.NewLogger()
	sniffer := sniff.NewDefaultSniffer()
	inflate := inflator.New()

	exitFuncs := []func(){
		func() { inflate.Close() },
	}

	signalsCh := make(chan os.Signal, 1)
	signal.Notify(signalsCh, os.Interrupt)

	go func() {
		for {
			select {
			case <-signalsCh:
				for _, f := range exitFuncs {
					f()
				}
				os.Exit(1)
			}
		}
	}()

	var credsFound int
	handler := func(logger lager.Logger, violation scanners.Violation) error {
		credsFound++
		output := fmt.Sprintf("%s ", red("[CRED]"))
		if violation.Line.Path == ".git/COMMIT_EDITMSG" {
			output = output + "line "
		} else {
			output = output + fmt.Sprintf("%s:", violation.Line.Path)
		}
		output = output + fmt.Sprintf("%d", violation.Line.LineNumber)
		if command.ShowCredentials {
			output = output + fmt.Sprintf(" [%s]", violation.Credential())
		}
		fmt.Println(output)

		return nil
	}

	if command.File != "" {
		fh, err := os.Open(command.File)
		if err != nil {
			log.Fatalln(err.Error())
		}

		br := bufio.NewReader(fh)
		if mime, isArchive := mimetype.IsArchive(logger, br); isArchive {
			inflateDir, err := ioutil.TempDir("", "cred-alert-cli")
			if err != nil {
				log.Fatalln(err.Error())
			}
			exitFuncs = append(exitFuncs, func() { os.RemoveAll(inflateDir) })

			violationsDir, err := ioutil.TempDir("", "cred-alert-cli-violations")
			if err != nil {
				log.Fatalln(err.Error())
			}

			archiveViolationHandler := func(logger lager.Logger, violation scanners.Violation) error {
				line := violation.Line
				credsFound++

				relPath, err := filepath.Rel(inflateDir, line.Path)
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

				fmt.Printf("%s %s:%d\n", red("[CRED]"), destPath, line.LineNumber)

				return nil
			}

			inflateStart := time.Now()
			fmt.Print("Inflating archive... ")
			err = inflate.Inflate(logger, mime, command.File, inflateDir)
			if err != nil {
				fmt.Printf("%s\n", red("FAILED"))
				log.Fatalln(err.Error())
			}
			fmt.Printf("%s\n", green("DONE"))

			scanStart := time.Now()
			dirScanner := dirscanner.New(archiveViolationHandler, sniffer)
			err = dirScanner.Scan(logger, inflateDir)
			if err != nil {
				log.Fatalln(err.Error())
			}

			fmt.Println()
			fmt.Println("Scan complete!")
			fmt.Println()
			fmt.Println("Time taken (inflating):", scanStart.Sub(inflateStart))
			fmt.Println("Time taken (scanning):", time.Since(scanStart))
			fmt.Println("Credentials found:", credsFound)
			fmt.Println()
			fmt.Println("Any archive inflation errors can be found in: ", inflate.LogPath())
		} else {
			scanFile(logger, handler, sniffer, br, command.File)
		}
	} else if command.Diff {
		handleDiff(logger, handler)
	} else {
		scanFile(logger, handler, sniffer, os.Stdin, "STDIN")
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

func scanFile(
	logger lager.Logger,
	handler sniff.ViolationHandlerFunc,
	sniffer sniff.Sniffer,
	f io.Reader,
	name string,
) {
	scanner := filescanner.New(f, name)
	sniffer.Sniff(logger, scanner, handler)
}

func handleDiff(logger lager.Logger, handler sniff.ViolationHandlerFunc) {
	logger.Session("handle-diff")
	scanner := diffscanner.NewDiffScanner(os.Stdin)
	sniffer := sniff.NewDefaultSniffer()

	sniffer.Sniff(logger, scanner, handler)
}

var twoWeeks = 14 * 24 * time.Hour

func warnIfOldExecutable() {
	exePath, err := osext.Executable()
	if err != nil {
		return
	}

	info, err := os.Stat(exePath)
	if err != nil {
		return
	}

	mtime := info.ModTime()

	if time.Now().Sub(mtime) > twoWeeks {
		fmt.Fprintln(os.Stderr, yellow("[WARN]"), "Executable is old! Please consider running `cred-alert-cli update`.")
	}
}

package commands

import (
	"bufio"
	"cred-alert/inflator"
	"cred-alert/kolsch"
	"cred-alert/mimetype"
	"cred-alert/scanners"
	"cred-alert/scanners/diffscanner"
	"cred-alert/scanners/dirscanner"
	"cred-alert/scanners/filescanner"
	"cred-alert/sniff"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/lager"
)

type ScanCommand struct {
	File string `short:"f" long:"file" description:"the file to scan" value-name:"FILE"`
	Diff bool   `long:"diff" description:"content to be scanned is a git diff"`
}

func (command *ScanCommand) Execute(args []string) error {
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
	handler := func(logger lager.Logger, line scanners.Line) error {
		credsFound++
		fmt.Printf("%s %s:%d\n", red("[CRED]"), line.Path, line.LineNumber)

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

			archiveViolationHandler := func(logger lager.Logger, line scanners.Line) error {
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
			fmt.Printf("%s (%s)\n", green("DONE"), time.Since(inflateStart))

			scanStart := time.Now()
			dirScanner := dirscanner.New(archiveViolationHandler, sniffer)
			err = dirScanner.Scan(logger, inflateDir)
			if err != nil {
				log.Fatalln(err.Error())
			}

			fmt.Println()
			fmt.Println("Scan complete!")
			fmt.Println()
			fmt.Println("Time taken:", time.Since(scanStart))
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

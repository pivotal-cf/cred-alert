package main

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
	"path/filepath"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"

	"github.com/jessevdk/go-flags"
	"github.com/mgutz/ansi"
)

type Opts struct {
	File string `short:"f" long:"file" description:"the file to scan" value-name:"FILE"`
	Diff bool   `long:"diff" description:"content to be scanned is a git diff"`
}

var red = ansi.ColorFunc("red+b")
var green = ansi.ColorFunc("green+b")

func main() {
	var opts Opts

	_, err := flags.ParseArgs(&opts, os.Args)
	if err != nil {
		os.Exit(1)
	}

	logger := kolsch.NewLogger()
	sniffer := sniff.NewDefaultSniffer()
	inflate := inflator.New()
	defer inflate.Close()

	var credsFound int
	handler := func(logger lager.Logger, line scanners.Line) error {
		credsFound++
		fmt.Printf("%s %s:%d\n", red("[CRED]"), line.Path, line.LineNumber)

		return nil
	}

	if opts.File != "" {
		fh, err := os.Open(opts.File)
		if err != nil {
			log.Fatalln(err.Error())
		}

		br := bufio.NewReader(fh)
		mime, isArchive := mimetype.IsArchive(logger, br)
		if isArchive {
			inflateDir, err := ioutil.TempDir("", "cred-alert-cli")
			if err != nil {
				log.Fatalln(err.Error())
			}
			defer os.RemoveAll(inflateDir)

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
			fmt.Printf("Inflating archive into %s\n", inflateDir)
			err = inflate.Inflate(logger, opts.File, inflateDir)
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
			if strings.HasPrefix(mime, "text") {
				scanFile(logger, handler, sniffer, br, opts.File)
			}
		}
	} else if opts.Diff {
		handleDiff(logger, handler, opts)

	} else {
		scanFile(logger, handler, sniffer, os.Stdin, "STDIN")
	}

	if credsFound > 0 {
		os.Exit(3)
	}
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

func handleDiff(logger lager.Logger, handler sniff.ViolationHandlerFunc, opts Opts) {
	logger.Session("handle-diff")
	scanner := diffscanner.NewDiffScanner(os.Stdin)
	sniffer := sniff.NewDefaultSniffer()

	sniffer.Sniff(logger, scanner, handler)
}

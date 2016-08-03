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
	"strings"

	"code.cloudfoundry.org/lager"

	"github.com/jessevdk/go-flags"
)

type Opts struct {
	File string `short:"f" long:"file" description:"the file to scan" value-name:"FILE"`
	Diff bool   `long:"diff" description:"content to be scanned is a git diff"`
}

func main() {
	var opts Opts

	_, err := flags.ParseArgs(&opts, os.Args)
	if err != nil {
		os.Exit(1)
	}

	logger := kolsch.NewLogger()
	sniffer := sniff.NewDefaultSniffer()

	if opts.File != "" {
		fh, err := os.Open(opts.File)
		if err != nil {
			log.Fatalln(err.Error())
		}

		destination, err := ioutil.TempDir("", "cred-alert-cli")
		if err != nil {
			log.Fatalln(err.Error())
		}

		defer os.RemoveAll(destination)

		br := bufio.NewReader(fh)
		mime, isArchive := mimetype.IsArchive(logger, br)
		if isArchive {
			err := inflator.RecursivelyExtractArchive(logger, opts.File, destination, false)
			if err != nil {
				log.Fatalln(err.Error())
			}

			dirScanner := dirscanner.New(handleViolation, sniffer)
			err = dirScanner.Scan(logger, destination)
			if err != nil {
				log.Fatalln(err.Error())
			}
		} else {
			if strings.HasPrefix(mime, "text") {
				scanFile(logger, sniffer, br, opts.File)
			}
		}
	} else if opts.Diff {
		handleDiff(logger, opts)
	} else {
		scanFile(logger, sniffer, os.Stdin, "STDIN")
	}
}

func handleViolation(logger lager.Logger, line scanners.Line) error {
	fmt.Printf("Line matches pattern! File: %s, Line Number: %d, Content: %s\n", line.Path, line.LineNumber, line.Content)

	return nil
}

func scanFile(
	logger lager.Logger,
	sniffer sniff.Sniffer,
	f io.Reader,
	name string,
) {
	scanner := filescanner.New(f, name)
	sniffer.Sniff(logger, scanner, handleViolation)
}

func handleDiff(logger lager.Logger, opts Opts) {
	logger.Session("handle-diff")
	diff, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		logger.Error("read-error", err)
	}

	scanner := diffscanner.NewDiffScanner(string(diff))
	sniffer := sniff.NewDefaultSniffer()

	sniffer.Sniff(logger, scanner, handleViolation)
}

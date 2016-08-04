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

	if opts.File != "" {
		start := time.Now()

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
			fmt.Printf("Inflating archive... ")
			err := inflate.Inflate(logger, opts.File, destination)
			if err != nil {
				fmt.Printf("%s\n", red("FAILED"))
				log.Fatalln(err.Error())
			}
			fmt.Printf("%s\n", green("DONE"))

			dirScanner := dirscanner.New(handleViolation, sniffer)
			err = dirScanner.Scan(logger, destination)
			if err != nil {
				log.Fatalln(err.Error())
			}

			duration := time.Since(start)

			fmt.Println()
			fmt.Println("Scan complete! Time taken:", duration)
			fmt.Println()
			fmt.Println("Any archive inflation errors can be found in: ", inflate.LogPath())
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
	fmt.Printf("%s %s:%d\n", red("[CRED]"), line.Path, line.LineNumber)

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

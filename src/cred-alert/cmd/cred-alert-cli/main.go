package main

import (
	"cred-alert/scanners"
	"cred-alert/scanners/filescanner"
	"cred-alert/sniff"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/jessevdk/go-flags"
	"github.com/pivotal-golang/lager"
)

type Opts struct {
	Directory string `short:"d" long:"directory" description:"the directory to scan" value-name:"DIR"`
	File      string `short:"f" long:"file" description:"the file to scan" value-name:"FILE"`
}

func main() {
	var opts Opts

	_, err := flags.ParseArgs(&opts, os.Args)
	if err != nil {
		os.Exit(1)
	}

	logger := lager.NewLogger("cred-alert-cli")
	logger.RegisterSink(lager.NewWriterSink(os.Stderr, lager.DEBUG))

	sniffer := sniff.NewDefaultSniffer()

	if opts.Directory != "" {
		scanDirectory(logger, sniffer, opts.Directory)
		os.Exit(0)
	}

	var f *os.File
	if opts.File != "" {
		var err error
		f, err = os.Open(opts.File)
		if err != nil {
			log.Fatalf("Failed to open file: %s", err.Error())
		}
		defer f.Close()
		scanner := filescanner.New(f, f.Name())
		sniffer.Sniff(logger, scanner, handleViolation)
		os.Exit(0)
	}

	scanner := filescanner.New(os.Stdin, os.Stdin.Name())
	sniffer.Sniff(logger, scanner, handleViolation)
	os.Exit(0)
}

func handleViolation(line scanners.Line) error {
	fmt.Printf("Line matches pattern! File: %s, Line Number: %d, Content: %s\n", line.Path, line.LineNumber, line.Content)

	return nil
}

func scanDirectory(logger lager.Logger, sniffer sniff.Sniffer, directoryPath string) {
	stat, err := os.Stat(directoryPath)
	if err != nil {
		log.Fatalf("Cannot read directory %s\n", directoryPath)
	}

	if !stat.IsDir() {
		log.Fatalf("%s is not a directory\n", directoryPath)
	}

	walkFunc := func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			fh, err := os.Open(path)
			if err != nil {
				return err
			}
			defer fh.Close()

			scanner := filescanner.New(fh, fh.Name())
			sniffer.Sniff(logger, scanner, handleViolation)
		}
		return nil
	}

	err = filepath.Walk(directoryPath, walkFunc)
	if err != nil {
		log.Fatalf("Error traversing directory: %v", err)
	}
}

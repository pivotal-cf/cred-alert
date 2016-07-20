package main

import (
	"cred-alert/scanners"
	"cred-alert/scanners/file"
	"cred-alert/sniff"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jessevdk/go-flags"
	"github.com/pivotal-golang/lager"
)

type Opts struct {
	Directory string `short:"d" long:"directory" description:"the directory to scan" value-name:"DIR"`
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
	} else {
		scanFile(logger, sniffer, os.Stdin)
	}
}

func handleViolation(line scanners.Line) error {
	fmt.Printf("Line matches pattern! File: %s, Line Number: %d, Content: %s\n", line.Path, line.LineNumber, line.Content)

	return nil
}

func scanFile(logger lager.Logger, sniffer sniff.Sniffer, fileHandle *os.File) {
	scanner := file.NewFileScanner(fileHandle)
	sniffer.Sniff(logger, scanner, handleViolation)
}

func scanDirectory(logger lager.Logger, sniffer sniff.Sniffer, directoryPath string) {
	if stat, err := os.Stat(directoryPath); err != nil {
		fmt.Fprintf(os.Stderr, "Cannot read directory %s\n", directoryPath)
		os.Exit(1)
	} else if !stat.IsDir() {
		fmt.Fprintf(os.Stderr, "%s is not a directory\n", directoryPath)
		os.Exit(1)
	}

	walkFunc := func(path string, info os.FileInfo, err error) error {
		fh, err := os.Open(path)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			scanFile(logger, sniffer, fh)
		}
		return nil
	}

	if err := filepath.Walk(directoryPath, walkFunc); err != nil {
		fmt.Fprintln(os.Stderr, "Error traversing directory: %v", err)
		os.Exit(1)
	}
}

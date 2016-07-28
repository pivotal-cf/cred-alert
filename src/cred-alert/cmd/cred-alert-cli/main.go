package main

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"cred-alert/mimetype"
	"cred-alert/scanners"
	"cred-alert/scanners/filescanner"
	"cred-alert/sniff"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

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
		scanFile(logger, sniffer, f, f.Name())
		os.Exit(0)
	}

	scanFile(logger, sniffer, os.Stdin, "STDIN")
}

func scanFile(logger lager.Logger, sniffer sniff.Sniffer, r io.Reader, name string) {
	br := bufio.NewReader(r)
	mime, ok := mimetype.IsArchive(br)
	if ok {
		switch mime {
		case "application/zip":
			r2, err := zip.OpenReader(name)
			if err != nil {
				log.Fatal(err.Error())
			}
			for i := range r2.File {
				rc, err := r2.File[i].Open()
				if err != nil {
					logger.Error("failed-to-open-file", err, lager.Data{
						"filename": name,
						"mimetype": mime,
					})
					continue
				}

				if r2.File[i].FileInfo().IsDir() {
					rc.Close()
					continue
				}

				scanFile(logger, sniffer, rc, r2.File[i].Name)
				rc.Close()
			}
		case "application/x-tar":
			r2 := tar.NewReader(br)

			for {
				header, err := r2.Next()
				if err != nil {
					if err == io.EOF {
						break
					}

					log.Fatal(err.Error())
				}

				if header.FileInfo().IsDir() {
					continue
				}

				scanFile(logger, sniffer, r2, header.Name)
			}
		case "application/gzip", "application/x-gzip":
			r2, err := gzip.NewReader(br)
			if err != nil {
				log.Fatal(err.Error())
			}

			scanFile(logger, sniffer, r2, name)

			r2.Close()
		default:
			panic(fmt.Sprintf("I don't know how to handle %s", mime))
		}
	}

	if strings.Contains(mime, "text") {
		scanner := filescanner.New(br, name)
		sniffer.Sniff(logger, scanner, handleViolation)
	}
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

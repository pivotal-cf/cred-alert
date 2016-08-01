package main

import (
	"bufio"
	"bytes"
	"cred-alert/mimetype"
	"cred-alert/scanners"
	"cred-alert/scanners/filescanner"
	"cred-alert/sniff"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"

	"github.com/jessevdk/go-flags"
)

type Opts struct {
	Directory string `short:"d" long:"directory" description:"the directory to scan" value-name:"DIR"`
	File      string `short:"f" long:"file" description:"the file to scan" value-name:"FILE"`
}

var sniffer = sniff.NewDefaultSniffer()

func main() {
	var opts Opts

	_, err := flags.ParseArgs(&opts, os.Args)
	if err != nil {
		os.Exit(1)
	}

	logger := lager.NewLogger("cred-alert-cli")
	logger.RegisterSink(lager.NewWriterSink(os.Stderr, lager.INFO))

	if opts.Directory != "" || opts.File != "" {
		destination := filepath.Join(os.TempDir(), fmt.Sprintf("%d", time.Now().Unix()))
		defer os.RemoveAll(destination)

		switch {
		case opts.Directory != "":
			handlePath(logger, opts.Directory, destination)
		case opts.File != "":
			handlePath(logger, opts.File, destination)
		}

		os.Exit(0)
	}

	scanFile(logger, os.Stdin, "STDIN")
}

func extractFile(mime, path, destination string) {
	err := os.MkdirAll(destination, 0755)
	if err != nil {
		panic(err.Error())
	}

	var cmd *exec.Cmd
	switch mime {
	case "application/zip":
		println(path)
		cmd = exec.Command("unzip", path, "-d", destination)
	case "application/x-tar":
		cmd = exec.Command("tar", "xf", path, "-C", destination)
	case "application/gzip", "application/x-gzip":
		fileName := filepath.Base(path)
		fileNameWithoutExt := fileName[:len(fileName)-len(filepath.Ext(fileName))]
		output, err := os.Create(filepath.Join(destination, fileNameWithoutExt))
		if err != nil {
			panic(err.Error())
		}
		defer output.Close()

		cmd = exec.Command("gunzip", "-c", path)
		cmd.Stdout = output
	default:
		panic(fmt.Sprintf("don't know how to extract %s", mime))
	}

	buf := &bytes.Buffer{}
	cmd.Stderr = buf
	err = cmd.Run()
	if err != nil {
		fmt.Printf("failed-to-run-cmd: %s\nStderr:\n%s\n", err.Error(), buf.String())
	}
}

func handleViolation(line scanners.Line) error {
	fmt.Printf("Line matches pattern! File: %s, Line Number: %d, Content: %s\n", line.Path, line.LineNumber, line.Content)

	return nil
}

func handlePath(logger lager.Logger, path, directoryPath string) {
	fi, err := os.Lstat(path)
	if err != nil {
		panic("could not lstat")
	}

	if fi.IsDir() {
		scanDirectory(logger, sniffer, path)
	} else {
		fh, err := os.Open(path)
		if err != nil {
			panic(err.Error())
		}
		br := bufio.NewReader(fh)
		mime, ok := mimetype.IsArchive(logger, br)
		if ok {
			archiveName := filepath.Base(fh.Name())
			destinationDir := filepath.Join(directoryPath, archiveName)
			extractFile(mime, fh.Name(), destinationDir)
			scanDirectory(logger, sniffer, destinationDir)
		} else {
			if strings.Contains(mime, "text") {
				scanFile(logger, br, fh.Name())
			}
		}
	}
}

func scanFile(logger lager.Logger, f io.Reader, name string) {
	scanner := filescanner.New(f, name)
	sniffer.Sniff(logger, scanner, handleViolation)
}

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func scanDirectory(
	logger lager.Logger,
	sniffer sniff.Sniffer,
	directoryPath string,
) {
	stat, err := os.Stat(directoryPath)
	if err != nil {
		log.Fatalf("Cannot read directory %s\n", directoryPath)
	}

	if !stat.IsDir() {
		log.Fatalf("%s is not a directory\n", directoryPath)
	}

	walkFunc := func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			if !info.Mode().IsRegular() {
				return nil
			}

			fh, err := os.Open(path)
			if err != nil {
				return err
			}
			defer fh.Close()

			handlePath(logger, path, directoryPath+randSeq(6))
		}
		return nil
	}

	err = filepath.Walk(directoryPath, walkFunc)
	if err != nil {
		log.Fatalf("Error traversing directory: %v", err)
	}
}

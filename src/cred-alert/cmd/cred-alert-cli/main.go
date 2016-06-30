package main

import (
	"cred-alert/scanners/file"
	"cred-alert/sniff"
	"fmt"
	"os"

	"github.com/pivotal-golang/lager"
)

func main() {
	logger := lager.NewLogger("cred-alert-cli")
	logger.RegisterSink(lager.NewWriterSink(os.Stderr, lager.DEBUG))
	scanner := file.NewFileScanner(os.Stdin)

	sniff.Sniff(logger, scanner, handleViolation)
}

func handleViolation(line sniff.Line) {
	fmt.Printf("Line matches pattern! File: %s, Line Number: %d, Content: %s\n", line.Path, line.LineNumber, line.Content)
}

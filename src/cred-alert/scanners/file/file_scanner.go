package file

import (
	"bufio"
	"cred-alert/sniff"
	"os"

	"github.com/pivotal-golang/lager"
)

type FileScanner struct {
	file        *os.File
	fileScanner *bufio.Scanner
}

func NewFileScanner(file *os.File) *FileScanner {
	return nil
}

func (s *FileScanner) Scan(logger lager.Logger) bool {
	return false
}

func (s *FileScanner) Line() *sniff.Line {
	return new(sniff.Line)
}

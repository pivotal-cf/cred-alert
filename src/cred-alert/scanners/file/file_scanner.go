package file

import (
	"bufio"
	"cred-alert/sniff"
	"os"

	"github.com/pivotal-golang/lager"
)

type fileScanner struct {
	path         string
	bufioScanner *bufio.Scanner
	lineNumber   int
}

func NewFileScanner(file *os.File) *fileScanner {
	bufioScanner := bufio.NewScanner(file)

	return &fileScanner{
		path:         file.Name(),
		bufioScanner: bufioScanner,
	}
}

func (s *fileScanner) Scan(logger lager.Logger) bool {
	logger = logger.Session("file-scanner")

	success := s.bufioScanner.Scan()

	if err := s.bufioScanner.Err(); err != nil {
		logger.Error("bufio-error", err)
		return false
	}

	if success {
		s.lineNumber++
	}
	return success
}

func (s *fileScanner) Line() *sniff.Line {
	return &sniff.Line{
		Content:    s.bufioScanner.Text(),
		LineNumber: s.lineNumber,
		Path:       s.path,
	}
}

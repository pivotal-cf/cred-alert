package filescanner

import (
	"bufio"
	"cred-alert/scanners"
	"io"

	"code.cloudfoundry.org/lager"
)

type fileScanner struct {
	path         string
	bufioScanner *bufio.Scanner
	lineNumber   int
}

func New(r io.Reader, filename string) *fileScanner {
	bufioScanner := bufio.NewScanner(r)
	return &fileScanner{
		path:         filename,
		bufioScanner: bufioScanner,
	}
}

func (s *fileScanner) Scan(logger lager.Logger) bool {
	logger = logger.Session("file-scanner").Session("scan")
	logger.Info("starting")

	success := s.bufioScanner.Scan()

	if err := s.bufioScanner.Err(); err != nil {
		logger.Error("bufio-error", err)
		return false
	}

	if success {
		s.lineNumber++
	}

	logger.Info("done")
	return success
}

func (s *fileScanner) Line(logger lager.Logger) *scanners.Line {
	lineNumber := s.lineNumber
	path := s.path

	logger = logger.Session("line", lager.Data{
		"liner-number": lineNumber,
		"path":         path,
	})
	logger.Info("starting")
	logger.Info("done")
	return &scanners.Line{
		Content:    s.bufioScanner.Text(),
		LineNumber: lineNumber,
		Path:       path,
	}
}

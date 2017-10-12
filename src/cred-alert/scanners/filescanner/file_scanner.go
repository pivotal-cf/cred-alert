package filescanner

import (
	"bufio"
	"cred-alert/scanners"
	"io"

	"code.cloudfoundry.org/lager"
)

type fileScanner struct {
	path        string
	reader      *bufio.Reader
	lineNumber  int
	currentLine []byte
	err         error
}

func New(r io.Reader, filename string) *fileScanner {
	return &fileScanner{
		path:   filename,
		reader: bufio.NewReader(r),
	}
}

func (s *fileScanner) Scan(logger lager.Logger) bool {
	logger = logger.Session("file-scanner").Session("scan")
	logger.Debug("starting")

	line, err := s.reader.ReadBytes('\n')
	if err != nil && err != io.EOF {
		logger.Error("bufio-error", err)
		s.err = err
		return false
	}

	if len(line) == 0 {
		return false
	}

	s.currentLine = line
	s.lineNumber++

	return true
}

func (s *fileScanner) Line(logger lager.Logger) *scanners.Line {
	lineNumber := s.lineNumber
	path := s.path

	logger = logger.Session("line", lager.Data{
		"liner-number": lineNumber,
		"path":         path,
	})
	logger.Debug("starting")
	logger.Debug("done")
	return &scanners.Line{
		Content:    s.currentLine,
		LineNumber: lineNumber,
		Path:       path,
	}
}

func (s *fileScanner) Err() error {
	return s.err
}

package diffscanner

import (
	"bufio"
	"cred-alert/scanners"
	"io"
	"regexp"
	"strconv"

	"code.cloudfoundry.org/lager"
)

var fileHeaderPattern = regexp.MustCompile(`^\+\+\+\s\w\/(.*)$`)
var addedLineRegexp = regexp.MustCompile(`^(?:\s|\+)(.*)`)
var hunkHeaderRegexp = regexp.MustCompile(`^@@.*\+(\d+),?\d+?\s@@`)

type DiffScanner struct {
	scanner           *bufio.Scanner
	content           []byte
	currentPath       string
	currentLineNumber int
}

func NewDiffScanner(diff io.Reader) *DiffScanner {
	return &DiffScanner{
		scanner: bufio.NewScanner(diff),
	}
}

func (d *DiffScanner) Scan(logger lager.Logger) bool {
	logger = logger.Session("diff-scanner").Session("scan")
	logger.Debug("starting")
	defer logger.Debug("done")

	for d.scanner.Scan() {
		line := d.scanner.Text()

		matches := fileHeaderPattern.FindStringSubmatch(line)
		if len(matches) == 2 {
			d.currentPath = matches[1]
			continue
		}

		matches = hunkHeaderRegexp.FindStringSubmatch(line)
		if len(matches) == 2 {
			startLine, err := strconv.Atoi(matches[1])
			if err != nil {
				logger.Error("failed", err)
				break
			}
			d.currentLineNumber = startLine - 1
			continue
		}

		matches = addedLineRegexp.FindStringSubmatch(line)
		if len(matches) == 2 {
			d.currentLineNumber++
			d.content = []byte(matches[1])
			return true
		}
	}

	return false
}

func (d *DiffScanner) Line(lager.Logger) *scanners.Line {
	return &scanners.Line{
		Content:    d.content,
		LineNumber: d.currentLineNumber,
		Path:       d.currentPath,
	}
}

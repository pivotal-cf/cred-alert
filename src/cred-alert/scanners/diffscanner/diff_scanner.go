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
var contextAddedLinePattern = regexp.MustCompile(`^(?:\s|\+)`)
var plusMinusSpacePattern = regexp.MustCompile(`^(?:\s|\+|\-)(.*)`)
var hunkHeaderRegexp = regexp.MustCompile(`^@@.*\+(\d+),?\d+?\s@@`)

type DiffScanner struct {
	scanner           *bufio.Scanner
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

		matches = contextAddedLinePattern.FindStringSubmatch(line)
		if len(matches) == 1 {
			d.currentLineNumber++
			return true
		}
	}

	return false
}

func (d *DiffScanner) Line(lager.Logger) *scanners.Line {
	var content []byte
	matches := plusMinusSpacePattern.FindSubmatchIndex(d.scanner.Bytes())
	if len(matches) == 4 {
		content = d.scanner.Bytes()[matches[2]:matches[3]]
	}

	return &scanners.Line{
		Content:    content,
		LineNumber: d.currentLineNumber,
		Path:       d.currentPath,
	}
}

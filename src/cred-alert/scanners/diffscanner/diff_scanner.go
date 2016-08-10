package diffscanner

import (
	"cred-alert/scanners"
	"regexp"
	"strconv"
	"strings"

	"code.cloudfoundry.org/lager"
)

var fileHeaderPattern = regexp.MustCompile(`^\+\+\+\s\w\/(.*)$`)
var contextAddedLinePattern = regexp.MustCompile(`^(?:\s|\+)`)
var plusMinusSpacePattern = regexp.MustCompile(`^(?:\s|\+|\-)(.*)`)
var hunkHeaderRegexp = regexp.MustCompile(`^@@.*\+(\d+),?\d+?\s@@`)

type DiffScanner struct {
	diff              []string
	cursor            int
	currentPath       string
	currentLineNumber int
}

func NewDiffScanner(diff string) *DiffScanner {
	return &DiffScanner{
		cursor: -1,
		diff:   strings.Split(diff, "\n"),
	}
}

func (d *DiffScanner) Scan(logger lager.Logger) bool {
	logger = logger.Session("diff-scanner").Session("scan")
	logger.Debug("starting")
	defer logger.Debug("done")

	for {
		d.cursor++
		if d.cursor >= len(d.diff) {
			break
		}

		rawLine := d.diff[d.cursor]

		matches := fileHeaderPattern.FindStringSubmatch(rawLine)
		if len(matches) == 2 {
			d.currentPath = matches[1]
			continue
		}

		matches = hunkHeaderRegexp.FindStringSubmatch(rawLine)
		if len(matches) == 2 {
			startLine, err := strconv.Atoi(matches[1])
			if err != nil {
				logger.Error("failed", err)
				break
			}
			d.currentLineNumber = startLine - 1
			continue
		}

		matches = contextAddedLinePattern.FindStringSubmatch(rawLine)
		if len(matches) == 1 {
			d.currentLineNumber++
			return true
		}
	}

	return false
}

func (d *DiffScanner) Line(logger lager.Logger) *scanners.Line {
	logger = logger.Session("line", lager.Data{
		"line-number": d.currentLineNumber,
		"path":        d.currentPath,
	})
	logger.Debug("starting")
	defer logger.Debug("done")

	var content string
	matches := plusMinusSpacePattern.FindStringSubmatch(d.diff[d.cursor])
	if len(matches) == 2 {
		content = matches[1]
	}

	return &scanners.Line{
		Content:    []byte(content),
		LineNumber: d.currentLineNumber,
		Path:       d.currentPath,
	}
}

package diffscanner

import (
	"cred-alert/scanners"
	"errors"
	"regexp"
	"strings"

	"code.cloudfoundry.org/lager"
)

type DiffScanner struct {
	diff              []string
	cursor            int
	currentHunk       *Hunk
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
	logger.Info("starting")

	// read information about hunk
	var isContentLine bool
	for !isContentLine {
		logger = logger.WithData(lager.Data{
			"line-number": d.cursor,
		})

		d.cursor++

		if d.cursor >= len(d.diff) {
			logger.Debug("passed-last-line")
			logger.Info("done")
			return false
		}

		logger.Debug("considering-line")
		rawLine := d.diff[d.cursor]

		d.scanHeader(logger, rawLine)
		isContentLine = d.scanHunk(logger, rawLine)
	}

	logger.Info("done")
	return true
}

func (d *DiffScanner) Line(logger lager.Logger) *scanners.Line {
	lineNumber := d.currentLineNumber
	path := d.currentHunk.path

	logger = logger.Session("line", lager.Data{
		"liner-number": lineNumber,
		"path":         path,
	})
	logger.Info("starting")

	content, err := diffContents(d.diff[d.cursor])
	if err != nil {
		logger.Error("setting content to ''", err)
	}

	logger.Info("done")
	return &scanners.Line{
		Content:    content,
		LineNumber: lineNumber,
		Path:       path,
	}
}

func (d *DiffScanner) scanHeader(logger lager.Logger, rawLine string) {
	logger = logger.Session("scan-header", lager.Data{
		"current-line-number": d.currentLineNumber,
	})
	logger.Info("starting")

	nextLineNumber := d.currentLineNumber + 1

	if !isInHeader(nextLineNumber, d.currentHunk) {
		logger.Info("done")
		return
	}

	path, err := fileHeader(rawLine, nextLineNumber, d.currentHunk)
	if err == nil {
		logger.Debug("detected-file-header")
		d.currentPath = path
		d.currentHunk = nil
	}

	startLine, length, err := hunkHeader(logger, rawLine)
	if err == nil {
		logger.Debug("detected-hunk-header")
		d.currentHunk = newHunk(d.currentPath, startLine, length)

		// the hunk header exists immeidately before the first line
		d.currentLineNumber = startLine - 1
	}
}

func (d *DiffScanner) scanHunk(logger lager.Logger, rawLine string) bool {
	logger = logger.Session("scan-hunk", lager.Data{
		"current-line-number": d.currentLineNumber,
	})
	logger.Info("starting")
	nextLineNumber := d.currentLineNumber + 1

	if isInHeader(nextLineNumber, d.currentHunk) {
		logger.Info("done")
		return false
	}

	if contextOrAddedLine(rawLine) {
		logger.Debug("detected-content-line")
		d.currentLineNumber = nextLineNumber
		logger.Info("done")
		return true
	}

	logger.Info("done")
	return false
}

var fileHeaderPattern = regexp.MustCompile(`^\+\+\+\s\w\/(.*)$`)
var contextAddedLinePattern = regexp.MustCompile(`^(\s|\+)`)
var plusMinusSpacePattern = regexp.MustCompile(`^(\s|\+|\-)(.*)`)

func fileHeader(rawLine string, currentLineNumber int, currentHunk *Hunk) (string, error) {
	if !isInHeader(currentLineNumber, currentHunk) {
		return "", errors.New("Still processing a hunk, not a file header")
	}
	return readFileHeader(rawLine)
}

func isInHeader(currentLineNumber int, currentHunk *Hunk) bool {
	if currentHunk == nil {
		return true
	}

	return currentHunk.endOfHunk(currentLineNumber)
}

func readFileHeader(line string) (string, error) {
	matches := fileHeaderPattern.FindStringSubmatch(line)
	if len(matches) < 2 {
		return "", errors.New("Not a path")
	}
	return matches[1], nil
}

func contextOrAddedLine(rawLine string) bool {
	matches := contextAddedLinePattern.FindStringSubmatch(rawLine)
	return len(matches) >= 2
}

func diffContents(rawLine string) (string, error) {
	matches := plusMinusSpacePattern.FindStringSubmatch(rawLine) // detect +, -, or <space>
	if len(matches) >= 3 {
		return matches[2], nil
	}

	return "", errors.New("Could not match content string")
}

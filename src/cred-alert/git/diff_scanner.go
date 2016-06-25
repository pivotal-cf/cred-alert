package git

import (
	"fmt"
	"strings"

	"github.com/pivotal-golang/lager"
)

type DiffScanner struct {
	diff              []string
	cursor            int
	currentHunk       *Hunk
	currentPath       string
	currentLineNumber int
}

func NewDiffScanner(diff string) *DiffScanner {
	d := new(DiffScanner)
	d.diff = strings.Split(diff, "\n")
	d.cursor = -1
	return d
}

func (d *DiffScanner) Scan(logger lager.Logger) bool {
	logger = logger.Session("diff-scanner")

	// read information about hunk
	for isContentLine := false; isContentLine == false; {
		logger = logger.WithData(lager.Data{
			"line-number": d.cursor,
		})

		d.cursor++

		if d.cursor >= len(d.diff) {
			logger.Debug("passed-last-line")
			return false
		}

		logger.Debug("considering-line")
		rawLine := d.diff[d.cursor]

		d.scanHeader(logger, rawLine)
		isContentLine = d.scanHunk(logger, rawLine)
	}

	logger.Debug("out-of-the-loop")
	return true
}

func (d *DiffScanner) scanHeader(logger lager.Logger, rawLine string) {
	nextLineNumber := d.currentLineNumber + 1

	logger = logger.Session("scan-header", lager.Data{
		"next-line-number": nextLineNumber,
	})

	if isInHeader(nextLineNumber, d.currentHunk) == false {
		return
	}

	path, err := fileHeader(rawLine, nextLineNumber, d.currentHunk)
	if err == nil {
		logger.Debug("detected-file-header")
		d.currentPath = path
		d.currentHunk = nil
	}

	startLine, length, err := hunkHeader(rawLine)
	if err == nil {
		logger.Debug("detected-hunk-header")
		d.currentHunk = newHunk(d.currentPath, startLine, length)

		// the hunk header exists immeidately before the first line
		d.currentLineNumber = startLine - 1
	}
}

func (d *DiffScanner) scanHunk(logger lager.Logger, rawLine string) bool {
	nextLineNumber := d.currentLineNumber + 1

	logger = logger.Session("scan-hunk", lager.Data{
		"next-line-number": nextLineNumber,
	})

	if isInHeader(nextLineNumber, d.currentHunk) {
		return false
	}

	if contextOrAddedLine(rawLine) {
		logger.Debug("detected-content-line")
		d.currentLineNumber = nextLineNumber
		return true
	}

	return false
}

func (d *DiffScanner) Line() *Line {
	line := new(Line)
	line.Path = d.currentHunk.path
	content, err := content(d.diff[d.cursor])
	if err == nil {
		line.Content = content
	} else {
		line.Content = ""
		fmt.Println(err)
	}
	line.LineNumber = d.currentLineNumber

	return line
}

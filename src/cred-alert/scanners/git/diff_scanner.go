package git

import (
	"cred-alert/scanners"
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
	logger = logger.Session("scan")
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

	startLine, length, err := hunkHeader(rawLine)
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

func (d *DiffScanner) Line() *scanners.Line {
	line := new(scanners.Line)
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

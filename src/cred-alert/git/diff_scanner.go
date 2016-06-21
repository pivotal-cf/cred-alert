package git

import (
	"fmt"
	"strings"
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

func (d *DiffScanner) Scan() bool {
	// read information about hunk

	for isContentLine := false; isContentLine == false; {
		d.cursor++
		if d.cursor >= len(d.diff) {
			// fmt.Println("\nWe have passed the last line, returning false...\n")
			return false
		}

		rawLine := d.diff[d.cursor]
		// fmt.Printf("\nConsidering line: <<<%s>>>\n", rawLine)

		d.scanHeader(rawLine)
		isContentLine = d.scanHunk(rawLine)
	}

	// fmt.Println("Out of the loop, returning true...")
	return true
}

func (d *DiffScanner) scanHeader(rawLine string) {
	nextLineNumber := d.currentLineNumber + 1

	if isInHeader(nextLineNumber, d.currentHunk) == false {
		return
	}

	path, err := fileHeader(rawLine, nextLineNumber, d.currentHunk)
	if err == nil {
		// fmt.Printf("Detected file header: %s\n", rawLine)
		d.currentPath = path
		d.currentHunk = nil
	}

	startLine, length, err := hunkHeader(rawLine)
	if err == nil {
		// fmt.Printf("Detected hunk header: %s\n", rawLine)
		d.currentHunk = newHunk(d.currentPath, startLine, length)
		// the hunk header exists immeidately before the first line
		d.currentLineNumber = startLine - 1
	}
}

func (d *DiffScanner) scanHunk(rawLine string) bool {
	nextLineNumber := d.currentLineNumber + 1

	if isInHeader(nextLineNumber, d.currentHunk) {
		return false
	}

	if contextOrAddedLine(rawLine) {
		// fmt.Printf("Detected content line: %s\n", rawLine)
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

	// fmt.Printf("%d: %s\n", line.LineNumber, line.Content)

	return line
}

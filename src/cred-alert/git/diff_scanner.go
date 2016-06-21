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
		nextLineNumber := d.currentLineNumber + 1
		if d.cursor >= len(d.diff) {
			// fmt.Println("\nWe have passed the last line, returning false...\n")
			return false
		}

		rawLine := d.diff[d.cursor]
		// fmt.Printf("\nConsidering line: <<<%s>>>\n", rawLine)

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

		if d.currentHunk != nil && d.currentHunk.endOfHunk(nextLineNumber) != true && contextOrAddedLine(rawLine) {
			// fmt.Printf("Detected content line: %s\n", rawLine)
			isContentLine = true
			d.currentLineNumber = nextLineNumber
		}
	}

	// fmt.Println("Out of the loop, returning true...")
	return true
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

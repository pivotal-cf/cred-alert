package git

import (
	"errors"
	"regexp"
)

var fileHeaderPattern *regexp.Regexp
var contextAddedLinePattern *regexp.Regexp
var plusMinusSpacePattern *regexp.Regexp

func init() {
	fileHeaderPattern = regexp.MustCompile(`^\+\+\+\s\w\/(.*)$`)
	contextAddedLinePattern = regexp.MustCompile(`^(\s|\+)`)
	plusMinusSpacePattern = regexp.MustCompile(`^(\s|\+|\-)(.*)`)
}

func isInHeader(currentLineNumber int, currentHunk *Hunk) bool {
	if currentHunk == nil {
		return true
	}

	if currentHunk != nil && currentHunk.endOfHunk(currentLineNumber) {
		return true
	}

	return false
}

func fileHeader(rawLine string, currentLineNumber int, currentHunk *Hunk) (string, error) {
	if currentHunk != nil && currentHunk.endOfHunk(currentLineNumber) == false {
		return "", errors.New("Still processing a hunk, not a file header")
	}
	return readFileHeader(rawLine)
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
	if len(matches) < 2 {
		// fmt.Printf("NOT a context or added line: <<<%s>>>\n", rawLine)
		return false
	}

	return true
}

func content(rawLine string) (string, error) {
	// detect +, -, or <space>
	matches := plusMinusSpacePattern.FindStringSubmatch(rawLine)
	if len(matches) >= 3 {
		return matches[2], nil
	}

	return "", errors.New("Could not match content string")
}

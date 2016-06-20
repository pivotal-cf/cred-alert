package git

import (
	"errors"
	"regexp"
)

type Line struct {
	Path       string
	LineNumber int
	Content    string

	action string
}

func fileHeader(diff []string, cursor int) (string, error) {
	// the very last line of a file could not be file header
	if cursor+1 >= len(diff) {
		return "", errors.New("Reached end of file")
	}

	rawLine := diff[cursor]

	matches := regexp.MustCompile(`^\+\+\+\s\w\/(.*)$`).FindStringSubmatch(rawLine)
	if len(matches) < 2 {
		return "", errors.New("Not a path")
	}

	// next line must be a hunk header
	nextRawLine := diff[cursor+1]
	_, _, err := hunkHeader(nextRawLine)
	if err != nil {
		return "", errors.New("Not a file header")
	}

	return matches[1], nil
}

func contextOrAddedLine(rawLine string) bool {
	matches := regexp.MustCompile(`^(\s|\+)`).FindStringSubmatch(rawLine)
	if len(matches) < 2 {
		// fmt.Printf("NOT a context or added line: <<<%s>>>\n", rawLine)
		return false
	}

	return true
}

func content(rawLine string) (string, error) {
	// detect hunk header, which is on the same line as the first line of content
	matches := regexp.MustCompile(`^@@\s-\d+,\d+\s+\+(\d+),\d+\s@@\s(.*)`).FindStringSubmatch(rawLine)
	if len(matches) >= 3 {
		return matches[2], nil
	}

	// detecdt +, -, or <space>
	matches = regexp.MustCompile(`^(\s|\+|\-)(.*)`).FindStringSubmatch(rawLine)
	if len(matches) >= 3 {
		return matches[2], nil
	}

	return "", errors.New("Could not match content string")
}

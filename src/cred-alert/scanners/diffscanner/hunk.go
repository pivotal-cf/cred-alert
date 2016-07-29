package diffscanner

import (
	"errors"
	"regexp"
	"strconv"

	"code.cloudfoundry.org/lager"
)

type Hunk struct {
	path      string
	startLine int
	length    int
}

var hunkHeaderRegexp = regexp.MustCompile(`^@@.*\+(\d+),?(\d+)?\s@@`)

func newHunk(path string, startLine int, length int) *Hunk {
	return &Hunk{
		path:      path,
		startLine: startLine,
		length:    length,
	}
}

func hunkHeader(logger lager.Logger, rawLine string) (int, int, error) {
	logger = logger.Session("hunk-header")
	logger.Info("starting")

	matches := hunkHeaderRegexp.FindStringSubmatch(rawLine)
	if len(matches) < 3 {
		logger.Debug("done")
		return 0, 0, errors.New("Not a hunk header")
	}

	startLine, err := strconv.Atoi(matches[1])
	if err != nil {
		logger.Error("failed", err)
		return 0, 0, err
	}

	var length int
	if matches[2] == "" {
		length = 1
	} else {
		var err error
		if length, err = strconv.Atoi(matches[2]); err != nil {
			logger.Error("failed", err)
			return 0, 0, err
		}
	}

	logger.Debug("done")
	return startLine, length, nil
}

func (h Hunk) endOfHunk(lineNumber int) bool {
	return lineNumber >= h.startLine+h.length
}

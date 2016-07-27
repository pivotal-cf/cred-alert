package textscanner

import (
	"cred-alert/scanners/filescanner"
	"cred-alert/sniff"
	"strings"
)

func New(text string) sniff.Scanner {
	reader := strings.NewReader(text)

	return filescanner.New(reader, "text")
}

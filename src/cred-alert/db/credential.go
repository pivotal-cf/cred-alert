package db

import "fmt"

type Credential struct {
	Model

	Scan   Scan
	ScanID uint

	Owner      string
	Repository string
	SHA        string
	Path       string
	LineNumber int
	MatchStart int
	MatchEnd   int
	Private    bool
}

func (c *Credential) Hash() string {
	return fmt.Sprintf("%s:%s:%s:%s:%d:%d:%d", c.Owner, c.Repository, c.SHA, c.Path, c.LineNumber, c.MatchStart, c.MatchEnd)
}

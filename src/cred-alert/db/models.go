package db

import (
	"fmt"
	"time"
)

type Model struct {
	ID        uint `gorm:"primary_key"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Commit struct {
	Model
	Owner      string
	Repository string
	SHA        string
}

type Scan struct {
	Model

	Type         string
	RulesVersion int

	ScanStart time.Time
	ScanEnd   time.Time

	Branch   string
	StartSHA string
	StopSHA  string

	Repository   *Repository
	RepositoryID *uint
	Fetch        *Fetch
	FetchID      *uint

	Credentials []Credential
}

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

type Repository struct {
	Model

	Cloned bool

	Name          string
	Owner         string
	Path          string
	SSHURL        string `gorm:"column:ssh_url"`
	Private       bool
	DefaultBranch string
	RawJSON       []byte `gorm:"column:raw_json"`

	FailedFetches int `gorm:"column:failed_fetches"`
	Disabled      bool

	FetchIntervalSeconds int `gorm:"column:fetch_interval"`

	CredentialCounts PropertyMap `gorm:"default:'{}'"`
}

type Fetch struct {
	Model
	Repository   Repository
	RepositoryID uint

	Path    string
	Changes []byte
}

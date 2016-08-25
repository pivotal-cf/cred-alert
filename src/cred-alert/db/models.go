package db

import "time"

type Model struct {
	ID        uint `gorm:"primary_key"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type DiffScan struct {
	Model
	Owner           string
	Repository      string
	FromCommit      string
	ToCommit        string
	CredentialFound bool
}

type Commit struct {
	Model
	Owner      string
	Repository string
	SHA        string
}

type Credential struct {
	Model
	Owner      string
	Repository string
	SHA        string
	Path       string
	LineNumber int

	ScanningMethod string
	RulesVersion   int
}

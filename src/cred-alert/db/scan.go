package db

import "time"

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

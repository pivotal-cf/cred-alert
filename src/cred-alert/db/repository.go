package db

import "encoding/json"

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

	CredentialCounts []byte
}

func (r Repository) GetCredentialCounts() map[string]int {
	credentialCounts := map[string]int{}

	json.Unmarshal(r.CredentialCounts, &credentialCounts)

	return credentialCounts
}

package db

import "github.com/jinzhu/gorm"

//go:generate counterfeiter . CredentialRepository

type CredentialRepository interface {
	ForScanWithID(int) ([]Credential, error)
}

type credentialRepository struct {
	db *gorm.DB
}

func NewCredentialRepository(db *gorm.DB) CredentialRepository {
	return &credentialRepository{db: db}
}

func (r *credentialRepository) ForScanWithID(scanID int) ([]Credential, error) {
	rows, err := r.db.DB().Query(`
    SELECT c.owner,
           c.repository,
           c.sha,
           c.path,
           c.line_number,
           c.match_start,
           c.match_end,
           c.private
    FROM credentials c
    WHERE c.scan_id = ?`, scanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var (
		credentials []Credential

		owner      string
		repository string
		sha        string
		path       string
		lineNumber int
		matchStart int
		matchEnd   int
		private    bool
	)

	for rows.Next() {
		scanErr := rows.Scan(
			&owner,
			&repository,
			&sha,
			&path,
			&lineNumber,
			&matchStart,
			&matchEnd,
			&private,
		)
		if scanErr != nil {
			return nil, scanErr
		}

		credentials = append(credentials, NewCredential(
			owner,
			repository,
			sha,
			path,
			lineNumber,
			matchStart,
			matchEnd,
			private,
		))
	}

	return credentials, nil
}

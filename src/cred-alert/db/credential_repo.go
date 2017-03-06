package db

import "github.com/jinzhu/gorm"

//go:generate counterfeiter . CredentialRepository

type CredentialRepository interface {
	ForScanWithID(int) ([]Credential, error)
	UniqueSHAsForRepoAndRulesVersion(Repository, int) ([]string, error)
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

		credentials = append(credentials, Credential{
			Owner:      owner,
			Repository: repository,
			SHA:        sha,
			Path:       path,
			LineNumber: lineNumber,
			MatchStart: matchStart,
			MatchEnd:   matchEnd,
			Private:    private,
		})
	}

	return credentials, nil
}

func (r *credentialRepository) UniqueSHAsForRepoAndRulesVersion(repo Repository, rulesVersion int) ([]string, error) {
	rows, err := r.db.DB().Query(`
    SELECT DISTINCT c.sha
		FROM   repositories r
					 JOIN scans s
						 ON s.repository_id = r.id
					 JOIN credentials c
						 ON c.scan_id = s.id
		WHERE  r.id = ?
					 AND s.rules_version = ?`, repo.ID, rulesVersion)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var shas []string
	var sha string
	for rows.Next() {
		scanErr := rows.Scan(&sha)
		if scanErr != nil {
			return nil, scanErr
		}
		shas = append(shas, sha)
	}

	return shas, nil
}

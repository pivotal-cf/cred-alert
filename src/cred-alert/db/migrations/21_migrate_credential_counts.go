package migrations

import (
	"encoding/json"

	"github.com/BurntSushi/migration"
)

func MigrateCredentialCounts(tx migration.LimitedTx) error {
	rows, err := tx.Query(`SELECT id, credential_counts FROM repositories`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var repositoryID uint
		var credentialCountsJSON []byte

		err = rows.Scan(&repositoryID, &credentialCountsJSON)
		if err != nil {
			return err
		}

		credentialCounts := map[string]uint{}
		json.Unmarshal(credentialCountsJSON, &credentialCounts)

		for branchName, credentialCount := range credentialCounts {
			_, err := tx.Exec(`INSERT INTO branches (
			  created_at,
			  updated_at,
			  repository_id,
			  name,
			  credential_count
			) VALUES (NOW(), NOW(), ?, ?, ?)`, repositoryID, branchName, credentialCount)

			if err != nil {
				return err
			}
		}
	}

	return nil
}

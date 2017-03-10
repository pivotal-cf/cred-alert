package migrations

import (
	"github.com/BurntSushi/migration"
)

func RemoveCredentialCountsAndRawJson(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE repositories
		  DROP COLUMN raw_json,
		  DROP COLUMN credential_counts
	`)


	return err
}

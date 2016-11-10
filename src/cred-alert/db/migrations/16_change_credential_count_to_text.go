package migrations

import "github.com/BurntSushi/migration"

func ChangeCredentialCountToText(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE repositories
		ADD COLUMN credential_counts longtext
	`)

	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE repositories
		DROP COLUMN credential_count
	`)

	return err
}

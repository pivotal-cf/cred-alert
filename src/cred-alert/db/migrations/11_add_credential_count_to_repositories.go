package migrations

import "github.com/BurntSushi/migration"

func AddCredentialCountToRepositories(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE repositories
			ADD COLUMN credential_count int
	`)

	if err != nil {
		return err
	}

	return nil
}

package migrations

import "github.com/BurntSushi/migration"

func AddFailedFetchesToRepositories(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE repositories
			ADD COLUMN failed_fetches int NOT NULL,
			ADD COLUMN disabled boolean NOT NULL
	`)

	if err != nil {
		return err
	}

	return nil
}

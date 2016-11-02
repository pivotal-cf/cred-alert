package migrations

import "github.com/BurntSushi/migration"

func AddFetchInterval(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE repositories
		ADD COLUMN fetch_interval int NOT NULL DEFAULT 86400
	`)

	return err
}

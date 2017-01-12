package migrations

import "github.com/BurntSushi/migration"

func RemoveFetchInterval(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE repositories
		DROP COLUMN fetch_interval
	`)

	return err
}

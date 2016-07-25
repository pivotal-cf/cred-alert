package migrations

import "github.com/BurntSushi/migration"

func DropCommitTimestamp(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    ALTER TABLE commits DROP COLUMN timestamp
	`)
	if err != nil {
		return err
	}

	return nil
}

package migrations

import "github.com/BurntSushi/migration"

func DropDiffScans(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    DROP TABLE diff_scans
	`)

	return err
}

package migrations

import "github.com/BurntSushi/migration"

func AddBranchToScans(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE scans
		ADD COLUMN branch varchar(255)
	`)

	return err
}

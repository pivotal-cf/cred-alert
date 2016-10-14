package migrations

import "github.com/BurntSushi/migration"

func AddStartSHAAndStopSHAToScans(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE scans
			ADD COLUMN start_sha varchar(40) NOT NULL,
			ADD COLUMN stop_sha varchar(40) NOT NULL
	`)

	if err != nil {
		return err
	}

	return nil
}

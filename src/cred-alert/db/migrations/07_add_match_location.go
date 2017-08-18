package migrations

import "github.com/BurntSushi/migration"

func AddMatchLocation(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE credentials
		ADD COLUMN match_start int DEFAULT 0,
		ADD COLUMN match_end int DEFAULT 0
	`)

	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE credentials
		SET match_start = 0, match_end = 0
	`)

	return err
}

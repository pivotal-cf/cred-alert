package migrations

import "github.com/BurntSushi/migration"

func AddAutoIncrement(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    ALTER TABLE diff_scans MODIFY COLUMN id int AUTO_INCREMENT
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
    ALTER TABLE commits MODIFY COLUMN id int AUTO_INCREMENT
	`)
	if err != nil {
		return err
	}

	return nil
}

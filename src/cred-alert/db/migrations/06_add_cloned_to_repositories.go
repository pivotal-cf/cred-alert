package migrations

import "github.com/BurntSushi/migration"

func AddClonedToRepositories(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE repositories
		ADD COLUMN cloned BOOLEAN DEFAULT false
	`)

	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE repositories
		SET cloned = true
	`)

	return err
}

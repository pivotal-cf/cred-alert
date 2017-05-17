package migrations

import (
	"github.com/BurntSushi/migration"
)

func AddIndexToRepositories(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE repositories ADD INDEX repositories_owner_name (owner, name)
	`)

	return err
}

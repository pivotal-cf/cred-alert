package migrations

import "github.com/BurntSushi/migration"

func UpdateNullCredentialCountsAgain(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		UPDATE repositories
		SET credential_counts = "{}"
		WHERE credential_counts LIKE "null"
	`)

	if err != nil {
		return err
	}

	return nil
}

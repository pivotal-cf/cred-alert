package migrations

import (
	"fmt"

	"github.com/BurntSushi/migration"
)

func RemoveFetchIDFromScan(tx migration.LimitedTx) error {
	var constraint string

	// We did not name the foreign key constraint when we created it, therefore
	// we need to fetch it in order to drop it.
	row := tx.QueryRow(`
		SELECT CONSTRAINT_NAME
		  FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE
			WHERE TABLE_NAME = 'scans'
			  AND COLUMN_NAME = 'fetch_id';
	`)

	err := row.Scan(&constraint)
	if err != nil {
		return err
	}

	// '`' is the quote character for mysql. '"' can be used if the ANSI_QUOTES
	// option is enabled, however we cannot guarantee that. Quoting the
	// identifier is necessary as we are using `fmt.Sprintf`. Templating using
	// '?' is not supported in this statement.
	_, err = tx.Exec(fmt.Sprintf("ALTER TABLE scans DROP FOREIGN KEY `%s`, DROP COLUMN fetch_id", constraint))

	return err
}

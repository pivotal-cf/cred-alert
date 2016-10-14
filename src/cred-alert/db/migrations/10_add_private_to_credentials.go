package migrations

import "github.com/BurntSushi/migration"

func AddPrivateToCredentials(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		ALTER TABLE credentials
			ADD COLUMN private bool NOT NULL
	`)

	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE credentials c
					 JOIN scans s
						 ON s.id = c.scan_id
					 JOIN repositories r
						 ON r.id = s.repository_id
			 SET c.private = r.private
	`)

	if err != nil {
		return err
	}

	return nil
}

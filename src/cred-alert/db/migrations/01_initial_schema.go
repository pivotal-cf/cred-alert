package migrations

import "github.com/BurntSushi/migration"

func InitialSchema(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    CREATE TABLE diff_scans (
      id int PRIMARY KEY,
      created_at datetime NOT NULL,
      updated_at datetime NOT NULL,
      owner text NOT NULL,
      repository text NOT NULL,
      from_commit varchar(40) NOT NULL,
      to_commit varchar(40) NOT NULL,
      credential_found bool
    )
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
    CREATE TABLE commits (
      id int PRIMARY KEY,
      created_at datetime NOT NULL,
      updated_at datetime NOT NULL,
      owner text NOT NULL,
      repository text NOT NULL,
      sha varchar(40) NOT NULL,
      timestamp datetime NOT NULL
    )
	`)
	return err
}

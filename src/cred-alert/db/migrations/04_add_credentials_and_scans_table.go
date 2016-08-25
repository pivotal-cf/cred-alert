package migrations

import "github.com/BurntSushi/migration"

func AddCredentialsAndScansTable(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    CREATE TABLE scans(
      id int PRIMARY KEY AUTO_INCREMENT,
      created_at datetime NOT NULL,
      updated_at datetime NOT NULL,
      type text NOT NULL,
			scan_start datetime NOT NULL,
			scan_end datetime NOT NULL,
			rules_version int NOT NULL
    )
	`)

	if err != nil {
		return err
	}

	_, err = tx.Exec(`
    CREATE TABLE credentials (
      id int PRIMARY KEY AUTO_INCREMENT,
			scan_id int,
      created_at datetime NOT NULL,
      updated_at datetime NOT NULL,
      owner text NOT NULL,
      repository text NOT NULL,
      sha varchar(40) NOT NULL,
      path text NOT NULL,
      line_number int NOT NULL,
			FOREIGN KEY (scan_id)
				REFERENCES scans(id)
				ON DELETE CASCADE
    )
	`)

	return err
}

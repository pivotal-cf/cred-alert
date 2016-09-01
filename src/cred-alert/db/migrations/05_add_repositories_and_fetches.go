package migrations

import "github.com/BurntSushi/migration"

func AddRepositoriesAndFetches(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		CREATE TABLE repositories
		(
			 id             INT PRIMARY KEY auto_increment,
			 name           VARCHAR(255) NOT NULL,
			 owner          VARCHAR(255) NOT NULL,
			 path           TEXT NOT NULL,
			 ssh_url        TEXT NOT NULL,
			 private        BOOLEAN NOT NULL,
			 default_branch TEXT NOT NULL,
			 raw_json       TEXT NOT NULL,
			 created_at     DATETIME NOT NULL,
			 updated_at     DATETIME NOT NULL,

			 UNIQUE (name, owner)
		)
	`)

	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE TABLE fetches
		(
			 id            INT PRIMARY KEY auto_increment,
			 repository_id INT NOT NULL,
			 path          TEXT NOT NULL,
			 changes       TEXT NOT NULL,
			 created_at    DATETIME NOT NULL,
			 updated_at    DATETIME NOT NULL,
			 FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE
		)
	`)

	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		ALTER TABLE scans
			ADD COLUMN repository_id INT NULL,
			ADD COLUMN fetch_id INT NULL,
			ADD FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE,
			ADD FOREIGN KEY (fetch_id) REFERENCES fetches(id) ON DELETE CASCADE
	`)

	return err
}

package migrations

import "github.com/BurntSushi/migration"

func AddBranches(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		CREATE TABLE branches (
			 id         INT PRIMARY KEY auto_increment,
			 created_at DATETIME NOT NULL,
			 updated_at DATETIME NOT NULL,

			 repository_id INT NOT NULL,

			 name             VARCHAR(255) CHARACTER SET utf8 COLLATE utf8_bin NOT NULL,
			 credential_count INT UNSIGNED NOT NULL,

			 FOREIGN KEY (repository_id) REFERENCES repositories(id) ON DELETE CASCADE,
			 UNIQUE KEY (repository_id, name)
		)
	`)

	return err
}

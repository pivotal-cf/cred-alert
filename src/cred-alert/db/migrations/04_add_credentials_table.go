package migrations

import "github.com/BurntSushi/migration"

func AddCredentialsTable(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    CREATE TABLE credentials (
      id int PRIMARY KEY AUTO_INCREMENT,
      created_at datetime NOT NULL,
      updated_at datetime NOT NULL,
      owner text NOT NULL,
      repository text NOT NULL,
      sha varchar(40) NOT NULL,
      path text NOT NULL,
      line_number int NOT NULL,
			scanning_method text NOT NULL,
			rules_version int NOT NULL
    )
	`)
	return err

}

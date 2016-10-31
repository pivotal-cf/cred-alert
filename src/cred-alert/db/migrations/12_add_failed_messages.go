package migrations

import "github.com/BurntSushi/migration"

func AddFailedMessages(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		CREATE TABLE failed_messages (
			id int PRIMARY KEY AUTO_INCREMENT,
			pubsub_message_id VARCHAR(255) NOT NULL DEFAULT "",
			retries int NOT NULL DEFAULT 0,
			dead_lettered bool NOT NULL DEFAULT false,
			UNIQUE(pubsub_message_id)
		)
	`)

	return err
}

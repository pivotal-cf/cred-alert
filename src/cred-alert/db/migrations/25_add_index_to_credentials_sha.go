package migrations

import "github.com/BurntSushi/migration"

func IndexCredentialsSha(tx migration.LimitedTx) error {
	_, err := tx.Exec("CREATE INDEX credentials_sha_idx ON credentials(sha);")
	return err
}

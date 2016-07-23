package migrations

import (
	"database/sql"
	"hash/crc32"
	"strings"
	"time"

	"github.com/BurntSushi/migration"
	"github.com/jinzhu/gorm"
	"github.com/pivotal-golang/lager"
)

func LockDBAndMigrate(logger lager.Logger, driver, dbURI string) (*gorm.DB, error) {
	var err error
	var lockDB *sql.DB

	for {
		lockDB, err = sql.Open(driver, dbURI)
		if err != nil {
			if strings.Contains(err.Error(), " dial ") {
				logger.Error("failed-to-open-db-retrying", err)
				time.Sleep(5 * time.Second)
				continue
			}
			return nil, err
		}

		break
	}

	lockName := crc32.ChecksumIEEE([]byte(driver + dbURI))

	for {
		_, err = lockDB.Exec(`SELECT GET_LOCK(?,10);`, lockName)
		if err != nil {
			logger.Error("failed-to-acquire-lock-retrying", err)
			time.Sleep(5 * time.Second)
			continue
		}

		logger.Info("migration-lock-acquired")
		_, err = migration.OpenWith(driver, dbURI, Migrations, migration.DefaultGetVersion, setVersion)
		if err != nil {
			logger.Fatal("failed-to-run-migrations", err)
		}

		_, err = lockDB.Exec(`SELECT RELEASE_LOCK(?)`, lockName)
		if err != nil {
			logger.Error("failed-to-release-lock", err)
		}

		lockDB.Close()
		break
	}

	return gorm.Open(driver, dbURI)
}

func setVersion(tx migration.LimitedTx, version int) error {
	_, err := tx.Exec("UPDATE migration_version SET version = ?", version)
	return err
}

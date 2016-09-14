package mysqlrunner

import (
	"cred-alert/db/migrations"
	"database/sql"
	"fmt"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	. "github.com/onsi/gomega"
)

type Runner struct {
	DBName   string
	dbConn   *sql.DB
	dbConfig *mysql.Config
}

func (runner *Runner) Setup() {
	runner.dbConfig = &mysql.Config{
		User:            "root",
		Net:             "tcp",
		Addr:            "127.0.0.1:3306",
		MultiStatements: true,
		Params: map[string]string{
			"charset":   "utf8",
			"parseTime": "True",
		},
	}
	dbConn, err := sql.Open("mysql", runner.DataSourceName())
	Expect(err).NotTo(HaveOccurred())

	runner.dbConn = dbConn

	_, err = runner.dbConn.Exec(fmt.Sprintf("CREATE DATABASE %s", runner.DBName))
	Expect(err).NotTo(HaveOccurred())

	runner.dbConfig.DBName = runner.DBName

	logger := lagertest.NewTestLogger("mysqlrunner-setup")
	lockDB, err := migrations.LockDBAndMigrate(logger, "mysql", runner.DataSourceName())
	Expect(err).NotTo(HaveOccurred())

	lockDB.Close()
}

func (runner *Runner) Teardown() {
	_, err := runner.dbConn.Exec(fmt.Sprintf("DROP DATABASE %s", runner.DBName))
	Expect(err).NotTo(HaveOccurred())

	runner.dbConn.Close()
}

func (runner *Runner) GormDB() (*gorm.DB, error) {
	database, err := gorm.Open("mysql", runner.DataSourceName())
	if err != nil {
		return nil, err
	}

	database.LogMode(false)
	return database, nil
}

func (runner *Runner) DataSourceName() string {
	return runner.dbConfig.FormatDSN()
}

func (runner *Runner) Truncate() {
	rows, err := runner.dbConn.Query(`
		SELECT TABLE_NAME
		FROM INFORMATION_SCHEMA.TABLES
		WHERE TABLE_SCHEMA IN (?)`, runner.DBName,
	)
	Expect(err).NotTo(HaveOccurred())
	defer rows.Close()

	for rows.Next() {
		var tableName string
		err := rows.Scan(&tableName)
		Expect(err).NotTo(HaveOccurred())

		_, err = runner.dbConn.Exec(fmt.Sprintf(`
			SET FOREIGN_KEY_CHECKS = 0;
			TRUNCATE TABLE %s.%s;
			SET FOREIGN_KEY_CHECKS = 1`,
			runner.DBName, tableName))
		Expect(err).NotTo(HaveOccurred())
	}
}

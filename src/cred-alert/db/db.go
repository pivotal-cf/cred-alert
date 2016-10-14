package db

import (
	"fmt"

	"github.com/go-sql-driver/mysql"
)

func NewDSN(username, password, dbName, hostname string, port int) string {
	dbConfig := &mysql.Config{
		User:            username,
		Passwd:          password,
		Net:             "tcp",
		DBName:          dbName,
		Addr:            fmt.Sprintf("%s:%d", hostname, port),
		MultiStatements: true,
		Params: map[string]string{
			"charset":   "utf8",
			"parseTime": "True",
		},
	}
	return dbConfig.FormatDSN()
}

func NewCredential(
	owner string,
	repository string,
	sha string,
	path string,
	lineNumber int,
	matchStart int,
	matchEnd int,
	private bool,
) Credential {
	return Credential{
		Owner:      owner,
		Repository: repository,
		SHA:        sha,
		Path:       path,
		LineNumber: lineNumber,
		MatchStart: matchStart,
		MatchEnd:   matchEnd,
		Private:    private,
	}
}

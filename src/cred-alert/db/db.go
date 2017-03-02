package db

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"github.com/go-sql-driver/mysql"
)

func NewDSN(username, password, dbName, hostname string, port int, serverName string, certificate tls.Certificate, caCertPool *x509.CertPool) string {
	mysql.RegisterTLSConfig("revok", &tls.Config{
		RootCAs:      caCertPool,
		Certificates: []tls.Certificate{certificate},
		ServerName:   serverName,
	})

	dbConfig := &mysql.Config{
		User:            username,
		Passwd:          password,
		Net:             "tcp",
		DBName:          dbName,
		Addr:            fmt.Sprintf("%s:%d", hostname, port),
		MultiStatements: true,
		TLSConfig:       "revok",
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

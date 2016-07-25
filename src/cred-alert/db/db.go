package db

import (
	"fmt"

	"github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	"github.com/pivotal-golang/lager"
)

func CreateDB(logger lager.Logger, uri string) (*gorm.DB, error) {
	return gorm.Open("mysql", uri)
}

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

//go:generate counterfeiter . CommitRepository

type CommitRepository interface {
	RegisterCommit(logger lager.Logger, commit *Commit) error
	IsCommitRegistered(logger lager.Logger, sha string) (bool, error)
	IsRepoRegistered(logger lager.Logger, owner, repo string) (bool, error)
}

type commitRepository struct {
	db *gorm.DB
}

func NewCommitRepository(db *gorm.DB) *commitRepository {
	return &commitRepository{
		db: db,
	}
}

func (c *commitRepository) RegisterCommit(logger lager.Logger, commit *Commit) error {
	logger = logger.Session("registering-commit", lager.Data{
		"owner":      commit.Owner,
		"repository": commit.Repository,
		"sha":        commit.SHA,
	})

	err := c.db.Save(commit).Error
	if err != nil {
		logger.Error("failed", err)
		return err
	}

	logger.Info("done")
	return nil
}

func (c *commitRepository) IsCommitRegistered(logger lager.Logger, sha string) (bool, error) {
	logger = logger.Session("finding-commit", lager.Data{
		"sha": sha,
	})

	var commits []Commit
	err := c.db.Where("SHA = ?", sha).First(&commits).Error
	if err != nil {
		logger.Error("failed", err)
	}

	return len(commits) == 1, err
}

func (c *commitRepository) IsRepoRegistered(logger lager.Logger, owner, repository string) (bool, error) {
	logger = logger.Session("finding-repo", lager.Data{
		"repository": repository,
	})

	var commits []Commit
	err := c.db.Where(&Commit{Owner: owner, Repository: repository}).First(&commits).Error
	if err != nil {
		logger.Error("error-finding-repo", err)
	}

	return len(commits) == 1, err
}

//go:generate counterfeiter . DiffScanRepository

type DiffScanRepository interface {
	SaveDiffScan(lager.Logger, *DiffScan) error
}

type diffScanRepository struct {
	db *gorm.DB
}

func NewDiffScanRepository(db *gorm.DB) *diffScanRepository {
	return &diffScanRepository{db: db}
}

func (d *diffScanRepository) SaveDiffScan(logger lager.Logger, diffScan *DiffScan) error {
	logger = logger.Session("saving-diffscan", lager.Data{
		"owner":            diffScan.Owner,
		"repository":       diffScan.Repository,
		"from-commit":      diffScan.FromCommit,
		"to-commit":        diffScan.ToCommit,
		"credential-found": diffScan.CredentialFound,
	})
	err := d.db.Save(diffScan).Error

	if err != nil {
		logger.Error("error-saving-diffscan", err)
	} else {
		logger.Info("successfully-saved-diffscan")
	}

	return err
}

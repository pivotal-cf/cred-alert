package db

import (
	"code.cloudfoundry.org/lager"
	"github.com/jinzhu/gorm"
)

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
	logger.Debug("starting")

	err := c.db.Save(commit).Error
	if err != nil {
		logger.Error("failed", err)
		return err
	}

	logger.Debug("done")
	return nil
}

func (c *commitRepository) IsCommitRegistered(logger lager.Logger, sha string) (bool, error) {
	logger = logger.Session("finding-commit", lager.Data{
		"sha": sha,
	})
	logger.Debug("starting")

	var commits []Commit
	err := c.db.Where("SHA = ?", sha).First(&commits).Error
	if err != nil {
		logger.Error("failed", err)
	}

	logger.Debug("done")
	return len(commits) == 1, err
}

func (c *commitRepository) IsRepoRegistered(logger lager.Logger, owner, repository string) (bool, error) {
	logger = logger.Session("finding-repo", lager.Data{
		"repository": repository,
	})
	logger.Debug("starting")

	var commits []Commit
	err := c.db.Where(&Commit{Owner: owner, Repository: repository}).First(&commits).Error
	if err != nil {
		logger.Error("failed", err)
	}

	logger.Debug("done")
	return len(commits) == 1, err
}

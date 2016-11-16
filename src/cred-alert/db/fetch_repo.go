package db

import (
	"code.cloudfoundry.org/lager"
	"github.com/jinzhu/gorm"
)

//go:generate counterfeiter . FetchRepository

type FetchRepository interface {
	RegisterFetch(lager.Logger, *Fetch) error
}

type fetchRepository struct {
	db *gorm.DB
}

func NewFetchRepository(db *gorm.DB) *fetchRepository {
	return &fetchRepository{db: db}
}

func (r *fetchRepository) RegisterFetch(logger lager.Logger, fetch *Fetch) error {
	logger = logger.Session("register-fetch", lager.Data{
		"path": fetch.Path,
	})
	logger.Debug("starting")

	tx := r.db.Begin()

	if fetch.Repository != nil {
		err := tx.Model(&fetch.Repository).Update("failed_fetches", 0).Error
		if err != nil {
			tx.Rollback()
			logger.Error("failed-to-update-failed-fetches", err)
			return err
		}
		fetch.RepositoryID = fetch.Repository.ID
		fetch.Repository = nil
	}

	err := tx.Save(fetch).Error
	if err != nil {
		tx.Rollback()
		logger.Error("failed-to-save-fetch", err)
		return err
	}

	tx.Commit()
	logger.Debug("done")
	return nil
}

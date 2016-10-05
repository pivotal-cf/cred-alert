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

	err := r.db.Save(fetch).Error
	if err != nil {
		logger.Error("failed", err)
		return err
	}

	logger.Debug("done")
	return nil
}

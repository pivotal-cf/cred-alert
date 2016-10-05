package db

import "github.com/jinzhu/gorm"

//go:generate counterfeiter . StatsRepository

type StatsRepository interface {
	RepositoryCount() (int, error)
	CredentialCount() (int, error)
	FetchCount() (int, error)
}

type statsRepository struct {
	db *gorm.DB
}

func NewStatsRepository(db *gorm.DB) StatsRepository {
	return &statsRepository{
		db: db,
	}
}

func (r *statsRepository) RepositoryCount() (int, error) {
	var count int
	err := r.db.Model(&Repository{}).Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *statsRepository) CredentialCount() (int, error) {
	var count int
	err := r.db.Model(&Credential{}).Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *statsRepository) FetchCount() (int, error) {
	var count int
	err := r.db.Model(&Fetch{}).Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

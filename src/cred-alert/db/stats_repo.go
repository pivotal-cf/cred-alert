package db

import "github.com/jinzhu/gorm"

//go:generate counterfeiter . StatsRepository

type StatsRepository interface {
	RepositoryCount() (int, error)
	DisabledRepositoryCount() (int, error)
	CredentialCount() (int, error)
	FetchCount() (int, error)
	DeadLetterCount() (int, error)
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

func (r *statsRepository) DisabledRepositoryCount() (int, error) {
	var count int
	err := r.db.DB().QueryRow(`
		SELECT count(1)
		FROM   repositories
		WHERE  disabled = true
	`).Scan(&count)

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

func (r *statsRepository) DeadLetterCount() (int, error) {
	var count int
	err := r.db.Model(&FailedMessage{}).Where("dead_lettered = ?", true).Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

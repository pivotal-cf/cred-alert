package db

import "github.com/jinzhu/gorm"

//go:generate counterfeiter . RepositoryRepository

type RepositoryRepository interface {
	FindOrCreate(*Repository) error
}

type repositoryRepository struct {
	db *gorm.DB
}

func NewRepositoryRepository(db *gorm.DB) *repositoryRepository {
	return &repositoryRepository{db: db}
}

func (r *repositoryRepository) FindOrCreate(repository *Repository) error {
	err := r.db.FirstOrCreate(repository, *repository).Error
	if err != nil {
		return err
	}
	return nil
}

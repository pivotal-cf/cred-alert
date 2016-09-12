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
	r2 := Repository{Name: repository.Name, Owner: repository.Owner}
	err := r.db.Where(r2).FirstOrCreate(repository).Error
	if err != nil {
		return err
	}
	return nil
}

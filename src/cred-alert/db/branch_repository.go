package db

import "github.com/jinzhu/gorm"

//go:generate counterfeiter . BranchRepository

type BranchRepository interface {
	GetBranches(repository Repository) ([]Branch, error)
	UpdateBranches(repository Repository, branches []Branch) error
}

type branchRepository struct {
	db *gorm.DB
}

func NewBranchRepository(db *gorm.DB) BranchRepository {
	return branchRepository{
		db: db,
	}
}

func (b branchRepository) GetBranches(repository Repository) ([]Branch, error) {
	branches := []Branch{}

	err := b.db.Where("repository_id = ?", repository.ID).Find(&branches).Error

	return branches, err
}

func (b branchRepository) UpdateBranches(repository Repository, branches []Branch) error {
	tx := b.db.Begin()
	defer tx.Rollback()

	err := tx.Where("repository_id = ?", repository.ID).Delete(Branch{}).Error
	if err != nil {
		return err
	}

	for _, branch := range branches {
		branch.RepositoryID = repository.ID

		err := tx.Create(&branch).Error
		if err != nil {
			return err
		}
	}

	return tx.Commit().Error
}

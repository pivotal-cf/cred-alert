package db

import "github.com/jinzhu/gorm"

//go:generate counterfeiter . BranchRepository

type BranchRepository interface {
	GetBranches(repository Repository) ([]Branch, error)
	UpdateBranches(repository Repository, branches []Branch) error

	GetCredentialCountByOwner() ([]OwnerCredentialCount, error)
}

type OwnerCredentialCount struct {
	Owner string
	CredentialCount int
}

type branchRepository struct {
	db *gorm.DB
}

func NewBranchRepository(db *gorm.DB) BranchRepository {
	return &branchRepository{
		db: db,
	}
}

func (b *branchRepository) GetBranches(repository Repository) ([]Branch, error) {
	branches := []Branch{}

	err := b.db.Where("repository_id = ?", repository.ID).Find(&branches).Error

	return branches, err
}

func (b *branchRepository) UpdateBranches(repository Repository, branches []Branch) error {
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

func (b *branchRepository) GetCredentialCountByOwner() ([]OwnerCredentialCount, error) {
	rows, err := b.db.DB().Query(`
		SELECT r.owner, SUM(b.credential_count)
		FROM repositories r
		JOIN branches b
		  ON r.id = b.repository_id
		GROUP BY r.owner
		ORDER BY r.owner
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := []OwnerCredentialCount{}

	for rows.Next() {
		var count OwnerCredentialCount

		err := rows.Scan(&count.Owner, &count.CredentialCount)
		if err != nil {
			return nil, err
		}

		counts = append(counts, count)
	}

	return counts, nil
}

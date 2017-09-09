package db

import (
	"errors"

	"code.cloudfoundry.org/lager"
	"github.com/jinzhu/gorm"

	"cred-alert/sniff"
)

//go:generate counterfeiter . RepositoryRepository

const FailedFetchThreshold = 3

type RepositoryRepository interface {
	Create(*Repository) error

	Update(*Repository) error

	Delete(*Repository) error

	Find(owner, name string) (Repository, bool, error)
	MustFind(owner, name string) (Repository, error)

	All() ([]Repository, error)
	Active() ([]Repository, error)
	AllForOrganization(string) ([]Repository, error)
	NotScannedWithVersion(int) ([]Repository, error)

	MarkAsCloned(owner, name, path string) error
	Reenable(owner, name string) error
	RegisterFailedFetch(lager.Logger, *Repository) error
}

type repositoryRepository struct {
	db *gorm.DB
}

func NewRepositoryRepository(db *gorm.DB) *repositoryRepository {
	return &repositoryRepository{
		db: db,
	}
}

func (r *repositoryRepository) Find(owner, name string) (Repository, bool, error) {
	repo, err := r.MustFind(owner, name)

	if err == gorm.ErrRecordNotFound {
		return Repository{}, false, nil
	} else if err != nil {
		return Repository{}, false, err
	}

	return repo, true, nil
}

func (r *repositoryRepository) MustFind(owner string, name string) (Repository, error) {
	var repo Repository
	err := r.db.Where("owner = ? AND name = ?", owner, name).First(&repo).Error

	if err != nil {
		return Repository{}, err
	}

	return repo, nil
}

func (r *repositoryRepository) Create(repository *Repository) error {
	return r.db.Create(repository).Error
}

func (r *repositoryRepository) Update(repository *Repository) error {
	return r.db.Model(&Repository{}).Where(
		Repository{Name: repository.Name, Owner: repository.Owner},
	).Updates(
		map[string]interface{}{
			"ssh_url":        repository.SSHURL,
			"default_branch": repository.DefaultBranch,
			"private":        repository.Private,
		},
	).Error
}

func (r *repositoryRepository) Delete(repository *Repository) error {
	return r.db.Delete(repository).Error
}

func (r *repositoryRepository) All() ([]Repository, error) {
	var existingRepositories []Repository
	err := r.db.Find(&existingRepositories).Error
	if err != nil {
		return nil, err
	}

	return existingRepositories, nil
}

func (r *repositoryRepository) Active() ([]Repository, error) {
	var repos []Repository
	err := r.db.Where("disabled = ? AND cloned = ?", false, true).Find(&repos).Error
	if err != nil {
		return nil, err
	}

	return repos, nil
}

func (r *repositoryRepository) AllForOrganization(owner string) ([]Repository, error) {
	var repositories []Repository
	err := r.db.Where("owner = ?", owner).Find(&repositories).Error
	if err != nil {
		return nil, err
	}

	return repositories, nil
}

func (r *repositoryRepository) MarkAsCloned(owner, name, path string) error {
	return r.db.Model(&Repository{}).Where(
		Repository{Name: name, Owner: owner},
	).Updates(
		map[string]interface{}{"cloned": true, "path": path},
	).Error
}

func (r *repositoryRepository) Reenable(owner, name string) error {
	return r.db.Model(&Repository{}).Where(
		Repository{Name: name, Owner: owner},
	).Update("disabled", false).Error
}

func (r *repositoryRepository) NotScannedWithVersion(version int) ([]Repository, error) {
	rows, err := r.db.Raw(`
    SELECT r.id
    FROM   scans s
           JOIN repositories r
             ON r.id = s.repository_id
           JOIN (SELECT repository_id      AS r_id,
                        MAX(rules_version) AS rules_version
                 FROM   scans
                 GROUP  BY repository_id
                ) latest_scans
             ON s.rules_version = latest_scans.rules_version
                AND s.repository_id = latest_scans.r_id
    WHERE  r.cloned = true
      AND  latest_scans.rules_version != ?`, sniff.RulesVersion).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		scanErr := rows.Scan(&id)
		if scanErr != nil {
			return nil, scanErr
		}
		ids = append(ids, id)
	}

	var repositories []Repository
	err = r.db.Model(&Repository{}).Where("id IN (?)", ids).Find(&repositories).Error
	if err != nil {
		return nil, err
	}

	return repositories, nil
}

func (r *repositoryRepository) RegisterFailedFetch(
	logger lager.Logger,
	repo *Repository,
) error {
	logger = logger.Session("register-failed-fetch", lager.Data{
		"ID": repo.ID,
	})

	tx, err := r.db.DB().Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.Exec(`
		UPDATE repositories
		SET failed_fetches = failed_fetches + 1
		WHERE id = ?
	`, repo.ID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		err := errors.New("repository could not be found")
		logger.Error("repository-not-found", err)
		return err
	}

	result, err = tx.Exec(`
		UPDATE repositories
		SET disabled = true
		WHERE id = ?
		AND failed_fetches >= ?
	`, repo.ID, FailedFetchThreshold)
	if err != nil {
		return err
	}

	rows, err = result.RowsAffected()
	if err != nil {
		return err
	}

	if rows > 0 {
		logger.Info("repository-disabled", lager.Data{
			"fetch-attempts": FailedFetchThreshold,
		})
	}

	return tx.Commit()
}

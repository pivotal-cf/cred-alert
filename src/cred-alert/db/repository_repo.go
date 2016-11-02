package db

import (
	"cred-alert/sniff"
	"errors"
	"fmt"
	"time"

	"code.cloudfoundry.org/lager"

	"github.com/jinzhu/gorm"
)

var (
	NeverBeenFetchedError = errors.New("Repository has never been fetched")
	NoChangesError        = errors.New("Repository has never been changed")
)

//go:generate counterfeiter . RepositoryRepository

type RepositoryRepository interface {
	FindOrCreate(*Repository) error
	Create(*Repository) error

	Find(owner string, name string) (Repository, error)

	All() ([]Repository, error)
	NotScannedWithVersion(int) ([]Repository, error)

	MarkAsCloned(string, string, string) error
	RegisterFailedFetch(lager.Logger, *Repository) error
	UpdateCredentialCount(*Repository, uint) error

	DueForFetch() ([]Repository, error)
	UpdateFetchInterval(*Repository, time.Duration) error
	LastActivity(*Repository) (time.Time, error)
}

type repositoryRepository struct {
	db *gorm.DB
}

func NewRepositoryRepository(db *gorm.DB) *repositoryRepository {
	return &repositoryRepository{
		db: db,
	}
}

func (r *repositoryRepository) Find(owner, name string) (Repository, error) {
	var repository Repository
	err := r.db.Where(Repository{Owner: owner, Name: name}).First(&repository).Error
	if err != nil {
		return Repository{}, err
	}
	return repository, nil
}

func (r *repositoryRepository) FindOrCreate(repository *Repository) error {
	r2 := Repository{Name: repository.Name, Owner: repository.Owner}
	return r.db.Where(r2).FirstOrCreate(repository).Error
}

func (r *repositoryRepository) Create(repository *Repository) error {
	return r.db.Create(repository).Error
}

func (r *repositoryRepository) All() ([]Repository, error) {
	var existingRepositories []Repository
	err := r.db.Find(&existingRepositories).Error
	if err != nil {
		return nil, err
	}

	return existingRepositories, nil
}

func (r *repositoryRepository) MarkAsCloned(owner, name, path string) error {
	return r.db.Model(&Repository{}).Where(
		Repository{Name: name, Owner: owner},
	).Updates(
		map[string]interface{}{"cloned": true, "path": path},
	).Error
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

const FailedFetchThreshold = 3

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
		e := errors.New(fmt.Sprintf("failed to fetch %d times", FailedFetchThreshold))
		logger.Error("repository-disabled", e)
	}

	return tx.Commit()
}

func (r *repositoryRepository) UpdateCredentialCount(repository *Repository, count uint) error {
	_, err := r.db.DB().Exec(`
		UPDATE repositories
		SET credential_count = ?
		WHERE id = ?
	`, count, repository.ID)

	return err
}

func (r *repositoryRepository) DueForFetch() ([]Repository, error) {
	ids := []int{}

	// old fetches
	rows, err := r.db.Raw(`
		SELECT r.id
		FROM repositories r
			JOIN (SELECT repository_id, MAX(created_at) AS last_activity
				FROM fetches
				WHERE changes != '{}'
				GROUP BY repository_id
			) activity
			ON activity.repository_id = r.id
	  WHERE UTC_TIMESTAMP() - INTERVAL r.fetch_interval SECOND > last_activity
		AND r.disabled = false
		AND r.cloned = true
	`).Rows()

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		scanErr := rows.Scan(&id)
		if scanErr != nil {
			return nil, scanErr
		}
		ids = append(ids, id)
	}

	// never been fetched
	rows, err = r.db.Raw(`
    SELECT r.id
    FROM   repositories r
           LEFT JOIN fetches f
             ON r.id = f.repository_id
    WHERE  f.repository_id IS NULL
	`).Rows()

	if err != nil {
		return nil, err
	}
	defer rows.Close()

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

func (r *repositoryRepository) LastActivity(repository *Repository) (time.Time, error) {
	var createdAt time.Time

	err := r.db.DB().QueryRow(`
		SELECT MAX(created_at)
		  FROM fetches
		  WHERE repository_id = ?
			AND changes != '{}'
	`, repository.ID).Scan(&createdAt)

	if err != nil {
		err = r.db.DB().QueryRow(`
		SELECT MAX(created_at)
		  FROM fetches
		  WHERE repository_id = ?
	`, repository.ID).Scan(&createdAt)

		if err != nil {
			return createdAt, NeverBeenFetchedError
		} else {
			return createdAt, NoChangesError
		}
	}

	return createdAt, nil
}

func (r *repositoryRepository) UpdateFetchInterval(repository *Repository, interval time.Duration) error {
	_, err := r.db.DB().Exec(`
		UPDATE repositories
		SET fetch_interval = ?
		WHERE id = ?
	`, int(interval.Seconds()), repository.ID)

	return err
}

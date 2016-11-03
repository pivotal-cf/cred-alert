package revok

import (
	"cred-alert/db"
	"math"
	"time"
)

//go:generate counterfeiter . FetchIntervalUpdater

type FetchIntervalUpdater interface {
	UpdateFetchInterval(*db.Repository) error
}

type fetchIntervalUpdater struct {
	repositoryRepository db.RepositoryRepository
	minimumInterval      time.Duration
	maximumInterval      time.Duration
}

func NewFetchIntervalUpdater(
	repositoryRepository db.RepositoryRepository,
	minimumInterval time.Duration,
	maximumInterval time.Duration,
) FetchIntervalUpdater {
	return &fetchIntervalUpdater{
		repositoryRepository: repositoryRepository,
		minimumInterval:      minimumInterval,
		maximumInterval:      maximumInterval,
	}
}

func (f *fetchIntervalUpdater) UpdateFetchInterval(repository *db.Repository) error {
	lastActivity, err := f.repositoryRepository.LastActivity(repository)
	var fetchInterval time.Duration

	if err == nil {
		duration := time.Now().Sub(lastActivity)

		c := math.Pow(float64(f.maximumInterval)/float64(f.minimumInterval), 1.0/3.0)

		switch {
		case duration < 24*time.Hour:
			fetchInterval = f.minimumInterval
		case duration < 168*time.Hour:
			fetchInterval = f.minimumInterval * time.Duration(c)
		case duration < 336*time.Hour:
			fetchInterval = f.minimumInterval * time.Duration(c*c)
		default:
			fetchInterval = f.maximumInterval
		}
	} else if err == db.NoChangesError {
		fetchInterval = f.maximumInterval
	} else {
		fetchInterval = f.minimumInterval
	}

	return f.repositoryRepository.UpdateFetchInterval(repository, fetchInterval)
}

package revok

import (
	"encoding/json"

	"code.cloudfoundry.org/lager"

	"cred-alert/db"
	"cred-alert/gitclient"
	"cred-alert/metrics"
)

//go:generate counterfeiter . ChangeFetcher

type ChangeFetcher interface {
	Fetch(logger lager.Logger, owner, name string, reenable bool) error
}

type changeFetcher struct {
	logger               lager.Logger
	gitClient            gitclient.Client
	notificationComposer NotificationComposer
	repositoryRepository db.RepositoryRepository
	fetchRepository      db.FetchRepository

	fetchTimer          metrics.Timer
	fetchSuccessCounter metrics.Counter
	fetchFailedCounter  metrics.Counter
	scanSuccessCounter  metrics.Counter
	scanFailedCounter   metrics.Counter
}

func NewChangeFetcher(
	logger lager.Logger,
	gitClient gitclient.Client,
	notificationComposer NotificationComposer,
	repositoryRepository db.RepositoryRepository,
	fetchRepository db.FetchRepository,
	emitter metrics.Emitter,
) ChangeFetcher {
	return &changeFetcher{
		logger:               logger,
		gitClient:            gitClient,
		notificationComposer: notificationComposer,
		repositoryRepository: repositoryRepository,
		fetchRepository:      fetchRepository,

		fetchTimer:          emitter.Timer("revok.change_discoverer.fetch_time"),
		fetchSuccessCounter: emitter.Counter("revok.change_discoverer.fetch.success"),
		fetchFailedCounter:  emitter.Counter("revok.change_discoverer.fetch.failed"),
		scanSuccessCounter:  emitter.Counter("revok.change_discoverer.scan.success"),
		scanFailedCounter:   emitter.Counter("revok.change_discoverer.scan.failed"),
	}
}

func (c *changeFetcher) Fetch(
	logger lager.Logger,
	owner string,
	name string,
	reenable bool,
) error {
	repoLogger := logger.WithData(lager.Data{
		"owner":      owner,
		"repository": name,
	})

	repo, found, err := c.repositoryRepository.Find(owner, name)
	if err != nil {
		repoLogger.Error("failed-to-find-repository", err)
		return err
	}

	shouldFetch, err := c.shouldFetch(repoLogger, repo, found, reenable)
	if err != nil {
		return err
	}

	if !shouldFetch {
		return nil
	}

	repoLogger = repoLogger.WithData(lager.Data{
		"path": repo.Path,
	})

	var changes map[string][]string
	var fetchErr error
	c.fetchTimer.Time(repoLogger, func() {
		changes, fetchErr = c.gitClient.Fetch(repo.Path)
	})

	if fetchErr != nil {
		repoLogger.Error("fetch-failed", fetchErr)
		c.fetchFailedCounter.Inc(repoLogger)

		registerErr := c.repositoryRepository.RegisterFailedFetch(repoLogger, &repo)
		if registerErr != nil {
			repoLogger.Error("failed-to-register-failed-fetch", registerErr)
		}

		return fetchErr
	}

	c.fetchSuccessCounter.Inc(repoLogger)

	bs, err := json.Marshal(changes)
	if err != nil {
		repoLogger.Error("failed-to-marshal-json", err)
		return err
	}

	fetch := db.Fetch{
		Repository: &repo,
		Path:       repo.Path,
		Changes:    bs,
	}

	err = c.fetchRepository.RegisterFetch(repoLogger, &fetch)
	if err != nil {
		repoLogger.Error("failed-to-save-fetch", err)
		return err
	}

	scannedOids := map[string]struct{}{}

	for branch, oids := range changes {
		err := c.notificationComposer.ScanAndNotify(
			logger,
			repo.Owner,
			repo.Name,
			scannedOids,
			branch,
			oids[1],
			oids[0],
		)
		if err != nil {
			c.scanFailedCounter.Inc(repoLogger)
		} else {
			c.scanSuccessCounter.Inc(repoLogger)
		}
	}

	return nil
}

func (c *changeFetcher) shouldFetch(repoLogger lager.Logger, repo db.Repository, found bool, reenable bool) (bool, error) {
	if !found {
		repoLogger.Info("skipping-fetch-of-unknown-repo")
		return false, nil
	}

	if repo.Disabled {
		if reenable {
			if err := c.repositoryRepository.Reenable(repo.Owner, repo.Name); err != nil {
				return false, err
			}
		} else {
			repoLogger.Info("skipping-fetch-of-disabled-repo")
			return false, nil
		}
	}

	if !repo.Cloned {
		repoLogger.Info("skipping-fetch-of-uncloned-repo")
		return false, nil
	}

	return true, nil
}

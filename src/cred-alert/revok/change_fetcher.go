package revok

import (
	"context"
	"encoding/json"

	"cloud.google.com/go/trace"
	"code.cloudfoundry.org/lager"

	"cred-alert/db"
	"cred-alert/gitclient"
	"cred-alert/metrics"
)

//go:generate counterfeiter . ChangeFetcher

type ChangeFetcher interface {
	Fetch(ctx context.Context, logger lager.Logger, owner, name string, reenable bool) error
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
	ctx context.Context,
	logger lager.Logger,
	owner string,
	name string,
	reenable bool,
) error {
	repoLogger := logger.WithData(lager.Data{
		"owner":      owner,
		"repository": name,
	})

	span := trace.FromContext(ctx).NewChild("Fetch")
	span.SetLabel("Owner", owner)
	span.SetLabel("Name", name)
	defer span.Finish()

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

	if err := c.registerFetchResult(repoLogger, repo, fetchErr, changes); err != nil {
		return err
	}

	return c.scanFetch(ctx, repoLogger, repo, changes)
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

func (c *changeFetcher) registerFetchResult(
	logger lager.Logger,
	repo db.Repository,
	fetchErr error,
	changes map[string][]string,
) error {
	if fetchErr != nil {
		logger.Error("fetch-failed", fetchErr)
		c.fetchFailedCounter.Inc(logger)

		registerErr := c.repositoryRepository.RegisterFailedFetch(logger, &repo)
		if registerErr != nil {
			logger.Error("failed-to-register-failed-fetch", registerErr)
		}

		return fetchErr
	}

	c.fetchSuccessCounter.Inc(logger)

	bs, err := json.Marshal(changes)
	if err != nil {
		logger.Error("failed-to-marshal-json", err)
		return err
	}

	fetch := db.Fetch{
		Repository: &repo,
		Path:       repo.Path,
		Changes:    bs,
	}

	err = c.fetchRepository.RegisterFetch(logger, &fetch)
	if err != nil {
		logger.Error("failed-to-save-fetch", err)
		return err
	}

	return nil
}

func (c *changeFetcher) scanFetch(
	ctx context.Context,
	logger lager.Logger,
	repo db.Repository,
	changes map[string][]string,
) error {
	scannedOids := map[string]struct{}{}

	for branch, oids := range changes {
		err := c.notificationComposer.ScanAndNotify(
			ctx,
			logger,
			repo.Owner,
			repo.Name,
			scannedOids,
			branch,
			oids[1],
			oids[0],
		)
		if err != nil {
			c.scanFailedCounter.Inc(logger)
		} else {
			c.scanSuccessCounter.Inc(logger)
		}
	}

	return nil
}

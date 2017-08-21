package revok

import (
	"context"
	"encoding/json"

	"cloud.google.com/go/trace"
	"code.cloudfoundry.org/lager"

	"cred-alert/db"
	"cred-alert/metrics"
)

//go:generate counterfeiter . ChangeFetcher

type ChangeFetcher interface {
	Fetch(ctx context.Context, logger lager.Logger, owner, name string, reenable bool) error
}

//go:generate counterfeiter . GitFetchClient
type GitFetchClient interface {
	Fetch(string) (map[string][]string, error)
}

type changeFetcher struct {
	logger               lager.Logger
	gitClient            GitFetchClient
	notificationComposer NotificationComposer
	repositoryRepository db.RepositoryRepository
	fetchRepository      db.FetchRepository

	fetchTimer     metrics.Timer
	fetchSuccesses metrics.Counter
	fetchFailures  metrics.Counter
	scanSuccesses  metrics.Counter
	scanFailures   metrics.Counter
}

func NewChangeFetcher(
	logger lager.Logger,
	gitClient GitFetchClient,
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

		fetchTimer:     emitter.Timer("revok.change_discoverer.fetch_time"),
		fetchSuccesses: emitter.Counter("revok.change_discoverer.fetch.success"),
		fetchFailures:  emitter.Counter("revok.change_discoverer.fetch.failed"),
		scanSuccesses:  emitter.Counter("revok.change_discoverer.scan.success"),
		scanFailures:   emitter.Counter("revok.change_discoverer.scan.failed"),
	}
}

func (c *changeFetcher) Fetch(
	ctx context.Context,
	logger lager.Logger,
	owner string,
	name string,
	reenable bool,
) error {
	logger = logger.Session("fetch-changes")

	repo, found, err := c.repositoryRepository.Find(owner, name)
	if err != nil {
		logger.Error("failed-to-find-repository", err)
		return err
	}

	shouldFetch, err := c.shouldFetch(logger, repo, found, reenable)
	if err != nil {
		return err
	}

	if !shouldFetch {
		return nil
	}

	logger = logger.WithData(lager.Data{
		"path": repo.Path,
	})

	var changes map[string][]string
	var fetchErr error
	span := trace.FromContext(ctx).NewChild("fetch-changes")
	c.fetchTimer.Time(logger, func() {
		changes, fetchErr = c.gitClient.Fetch(repo.Path)
	})
	span.Finish()

	if err := c.registerFetchResult(logger, repo, fetchErr, changes); err != nil {
		return err
	}

	return c.scanFetch(ctx, logger, repo, changes)
}

func (c *changeFetcher) shouldFetch(logger lager.Logger, repo db.Repository, found bool, reenable bool) (bool, error) {
	if !found {
		logger.Info("skipping-fetch-of-unknown-repo")
		return false, nil
	}

	if repo.Disabled {
		if reenable {
			if err := c.repositoryRepository.Reenable(repo.Owner, repo.Name); err != nil {
				return false, err
			}
		} else {
			logger.Info("skipping-fetch-of-disabled-repo")
			return false, nil
		}
	}

	if !repo.Cloned {
		logger.Info("skipping-fetch-of-uncloned-repo")
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
		c.fetchFailures.Inc(logger)

		registerErr := c.repositoryRepository.RegisterFailedFetch(logger, &repo)
		if registerErr != nil {
			logger.Error("failed-to-register-failed-fetch", registerErr)
		}

		return fetchErr
	}

	c.fetchSuccesses.Inc(logger)

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
			c.scanFailures.Inc(logger)
		} else {
			c.scanSuccesses.Inc(logger)
		}
	}

	return nil
}

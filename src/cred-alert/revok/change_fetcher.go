package revok

import (
	"cred-alert/db"
	"cred-alert/gitclient"
	"cred-alert/kolsch"
	"cred-alert/metrics"
	"encoding/json"

	git "github.com/libgit2/git2go"

	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . ChangeFetcher

type ChangeFetcher interface {
	Fetch(logger lager.Logger, owner string, name string) error
}

type changeFetcher struct {
	logger               lager.Logger
	gitClient            gitclient.Client
	scanner              Scanner
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
	scanner Scanner,
	repositoryRepository db.RepositoryRepository,
	fetchRepository db.FetchRepository,
	emitter metrics.Emitter,
) ChangeFetcher {
	return &changeFetcher{
		logger:               logger,
		gitClient:            gitClient,
		scanner:              scanner,
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
) error {
	repoLogger := logger.WithData(lager.Data{
		"owner":      owner,
		"repository": name,
	})

	repo, err := c.repositoryRepository.Find(owner, name)
	if err != nil {
		repoLogger.Error("failed-to-find-repository", err)
		return err
	}

	if repo.Disabled {
		repoLogger.Info("skipping-fetch-of-disabled-repo")
		return nil
	}

	repoLogger = repoLogger.WithData(lager.Data{
		"path": repo.Path,
	})

	var changes map[string][]*git.Oid
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

	scannedOids := map[git.Oid]struct{}{}

	for branch, oids := range changes {
		err := c.scanner.Scan(
			kolsch.NewLogger(),
			repo.Owner,
			repo.Name,
			scannedOids,
			branch,
			oids[1].String(),
			oids[0].String(),
		)
		if err != nil {
			c.scanFailedCounter.Inc(repoLogger)
		} else {
			c.scanSuccessCounter.Inc(repoLogger)
		}
	}

	return nil
}

package revok

import (
	"context"
	"cred-alert/db"
	"cred-alert/gitclient"
	"cred-alert/kolsch"
	"cred-alert/metrics"
	"encoding/json"
	"os"
	"time"

	git "github.com/libgit2/git2go"
	"github.com/tedsuo/ifrit"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
)

type ChangeDiscoverer struct {
	logger               lager.Logger
	gitClient            gitclient.Client
	clock                clock.Clock
	interval             time.Duration
	scanner              Scanner
	repositoryRepository db.RepositoryRepository
	fetchRepository      db.FetchRepository

	fetchTimer             metrics.Timer
	fetchedRepositoryGauge metrics.Gauge
	runCounter             metrics.Counter
	fetchSuccessCounter    metrics.Counter
	fetchFailedCounter     metrics.Counter
	scanSuccessCounter     metrics.Counter
	scanFailedCounter      metrics.Counter
}

func NewChangeDiscoverer(
	logger lager.Logger,
	gitClient gitclient.Client,
	clock clock.Clock,
	interval time.Duration,
	scanner Scanner,
	repositoryRepository db.RepositoryRepository,
	fetchRepository db.FetchRepository,
	emitter metrics.Emitter,
) ifrit.Runner {
	return &ChangeDiscoverer{
		logger:               logger,
		gitClient:            gitClient,
		clock:                clock,
		interval:             interval,
		scanner:              scanner,
		repositoryRepository: repositoryRepository,
		fetchRepository:      fetchRepository,

		fetchTimer:             emitter.Timer("revok.change_discoverer.fetch_time"),
		fetchedRepositoryGauge: emitter.Gauge("revok.change_discoverer.repositories_to_fetch"),
		runCounter:             emitter.Counter("revok.change_discoverer.run"),
		fetchSuccessCounter:    emitter.Counter("revok.change_discoverer.fetch.success"),
		fetchFailedCounter:     emitter.Counter("revok.change_discoverer.fetch.failed"),
		scanSuccessCounter:     emitter.Counter("revok.change_discoverer.scan.success"),
		scanFailedCounter:      emitter.Counter("revok.change_discoverer.scan.failed"),
	}
}

func (c *ChangeDiscoverer) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := c.logger.Session("change-discoverer")
	logger.Info("started")

	close(ready)

	timer := c.clock.NewTicker(c.interval)
	firstTickCh := c.clock.NewTimer(0).C()

	ctx, cancel := context.WithCancel(context.Background())

	for {
		select {
		case <-firstTickCh:
			firstTickCh = nil
			c.work(ctx, logger)
		case <-timer.C():
			c.work(ctx, logger)
		case <-signals:
			cancel()
			logger.Info("done")
			timer.Stop()
			return nil
		}
	}

	return nil
}

func (c *ChangeDiscoverer) work(ctx context.Context, logger lager.Logger) {
	c.runCounter.Inc(logger)

	repos, err := c.repositoryRepository.NotFetchedSince(c.clock.Now().Add(-c.interval))
	if err != nil {
		logger.Error("failed-getting-repos", err)
		return
	}

	c.fetchedRepositoryGauge.Update(logger, float32(len(repos)))

	if len(repos) == 0 {
		return
	}

	err = c.fetch(logger, repos[0])
	if err != nil {
		return
	}

	if len(repos) > 1 {
		repoFetchDelay := time.Duration(c.interval.Nanoseconds()/int64(len(repos))) * time.Nanosecond
		waitCh := c.clock.NewTicker(repoFetchDelay).C()
		errCh := make(chan error)

		for _, repo := range repos[1:] {
			<-waitCh

			go func() { errCh <- c.fetch(logger, repo) }()

			select {
			case <-errCh:
			case <-ctx.Done():
			}
		}
	}

	return
}

func (c *ChangeDiscoverer) fetch(
	logger lager.Logger,
	repo db.Repository,
) error {
	repoLogger := logger.WithData(lager.Data{
		"owner":      repo.Owner,
		"repository": repo.Name,
		"path":       repo.Path,
	})

	var changes map[string][]*git.Oid
	var fetchErr error
	c.fetchTimer.Time(repoLogger, func() {
		changes, fetchErr = c.gitClient.Fetch(repo.Path)
	})

	if fetchErr != nil {
		repoLogger.Error("failed-to-fetch", fetchErr)
		c.fetchFailedCounter.Inc(repoLogger)
		return fetchErr
	}

	c.fetchSuccessCounter.Inc(repoLogger)

	bs, err := json.Marshal(changes)
	if err != nil {
		repoLogger.Error("failed-to-marshal-json", err)
		return err
	}

	fetch := db.Fetch{
		Repository: repo,
		Path:       repo.Path,
		Changes:    bs,
	}

	err = c.fetchRepository.SaveFetch(repoLogger, &fetch)
	if err != nil {
		repoLogger.Error("failed-to-save-fetch", err)
		return err
	}

	quietLogger := kolsch.NewLogger()

	for _, oids := range changes {
		err := c.scanner.Scan(quietLogger, repo.Owner, repo.Name, oids[1].String(), oids[0].String())
		if err != nil {
			c.scanFailedCounter.Inc(repoLogger)
		} else {
			c.scanSuccessCounter.Inc(repoLogger)
		}
	}

	return nil
}

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

//go:generate counterfeiter . ChangeDiscoverer

type ChangeDiscoverer interface {
	ifrit.Runner

	Fetch(lager.Logger, db.Repository) error
}

type changeDiscoverer struct {
	logger               lager.Logger
	gitClient            gitclient.Client
	clock                clock.Clock
	interval             time.Duration
	scanner              Scanner
	repositoryRepository db.RepositoryRepository
	fetchRepository      db.FetchRepository
	fetchIntervalUpdater FetchIntervalUpdater

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
	fetchIntervalUpdater FetchIntervalUpdater,
	emitter metrics.Emitter,
) ChangeDiscoverer {
	return &changeDiscoverer{
		logger:               logger,
		gitClient:            gitClient,
		clock:                clock,
		interval:             interval,
		scanner:              scanner,
		repositoryRepository: repositoryRepository,
		fetchRepository:      fetchRepository,
		fetchIntervalUpdater: fetchIntervalUpdater,

		fetchTimer:             emitter.Timer("revok.change_discoverer.fetch_time"),
		fetchedRepositoryGauge: emitter.Gauge("revok.change_discoverer.repositories_to_fetch"),
		runCounter:             emitter.Counter("revok.change_discoverer.run"),
		fetchSuccessCounter:    emitter.Counter("revok.change_discoverer.fetch.success"),
		fetchFailedCounter:     emitter.Counter("revok.change_discoverer.fetch.failed"),
		scanSuccessCounter:     emitter.Counter("revok.change_discoverer.scan.success"),
		scanFailedCounter:      emitter.Counter("revok.change_discoverer.scan.failed"),
	}
}

func (c *changeDiscoverer) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := c.logger.Session("change-discoverer")
	logger.Info("started")

	close(ready)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c.work(signals, cancel, logger)

	timer := c.clock.NewTicker(c.interval)

	for {
		select {
		case <-timer.C():
			c.work(signals, cancel, logger)
		case <-ctx.Done():
			logger.Info("done")
			timer.Stop()
			return nil
		case <-signals:
			cancel()
		}
	}
}

func (c *changeDiscoverer) work(signals <-chan os.Signal, cancel context.CancelFunc, logger lager.Logger) {
	c.runCounter.Inc(logger)

	repos, err := c.repositoryRepository.DueForFetch()
	if err != nil {
		logger.Error("failed-getting-repos", err)
		return
	}

	c.fetchedRepositoryGauge.Update(logger, float32(len(repos)))

	if len(repos) > 0 {
		repoFetchDelay := time.Duration(c.interval.Nanoseconds()/int64(len(repos))) * time.Nanosecond
		waitCh := c.clock.NewTicker(repoFetchDelay).C()

		for i := range repos {
			select {
			case <-signals:
				cancel()
				return
			default:
				c.Fetch(logger, repos[i])
				if i < len(repos)-1 {
					<-waitCh
				}
			}
		}
	}
}

func (c *changeDiscoverer) Fetch(
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
		Repository: repo,
		Path:       repo.Path,
		Changes:    bs,
	}

	err = c.fetchRepository.RegisterFetch(repoLogger, &fetch)
	if err != nil {
		repoLogger.Error("failed-to-save-fetch", err)
		return err
	}

	c.fetchIntervalUpdater.UpdateFetchInterval(&repo)

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

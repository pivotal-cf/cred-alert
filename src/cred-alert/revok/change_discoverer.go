package revok

import (
	"context"
	"cred-alert/db"
	"cred-alert/gitclient"
	"cred-alert/kolsch"
	"cred-alert/metrics"
	"cred-alert/scanners"
	"cred-alert/scanners/diffscanner"
	"cred-alert/sniff"
	"encoding/json"
	"os"
	"strings"
	"time"

	git "github.com/libgit2/git2go"
	"github.com/tedsuo/ifrit"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
)

type ChangeDiscoverer struct {
	logger               lager.Logger
	workdir              string
	gitClient            gitclient.Client
	clock                clock.Clock
	interval             time.Duration
	sniffer              sniff.Sniffer
	repositoryRepository db.RepositoryRepository
	fetchRepository      db.FetchRepository
	scanRepository       db.ScanRepository

	fetchTimer             metrics.Timer
	fetchedRepositoryGauge metrics.Gauge
	runCounter             metrics.Counter
	failedCounter          metrics.Counter
	failedScanCounter      metrics.Counter
	failedDiffCounter      metrics.Counter
	successCounter         metrics.Counter
}

func NewChangeDiscoverer(
	logger lager.Logger,
	workdir string,
	gitClient gitclient.Client,
	clock clock.Clock,
	interval time.Duration,
	sniffer sniff.Sniffer,
	repositoryRepository db.RepositoryRepository,
	fetchRepository db.FetchRepository,
	scanRepository db.ScanRepository,
	emitter metrics.Emitter,
) ifrit.Runner {
	return &ChangeDiscoverer{
		logger:               logger,
		workdir:              workdir,
		gitClient:            gitClient,
		clock:                clock,
		interval:             interval,
		sniffer:              sniffer,
		repositoryRepository: repositoryRepository,
		fetchRepository:      fetchRepository,
		scanRepository:       scanRepository,

		fetchTimer:             emitter.Timer("revok.fetch_time"),
		fetchedRepositoryGauge: emitter.Gauge("revok.fetched_repositories"),
		runCounter:             emitter.Counter("revok.change_discoverer_runs"),
		successCounter:         emitter.Counter("revok.change_discoverer_success"),
		failedCounter:          emitter.Counter("revok.change_discoverer_failed"),
		failedDiffCounter:      emitter.Counter("revok.change_discoverer_failed_diffs"),
		failedScanCounter:      emitter.Counter("revok.change_discoverer_failed_scans"),
	}
}

func (c *ChangeDiscoverer) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := c.logger.Session("change-discoverer")
	logger.Info("started")

	close(ready)

	c.runCounter.Inc(logger)

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
		c.failedCounter.Inc(logger)
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
			case err := <-errCh:
				if err != nil {
					c.failedCounter.Inc(logger)
				}
			case <-ctx.Done():
				return
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
		return fetchErr
	}

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
		diff, err := c.gitClient.Diff(repo.Path, oids[0], oids[1])
		if err != nil {
			repoLogger.Error("failed-to-get-diff", err, lager.Data{
				"from": oids[0].String(),
				"to":   oids[1].String(),
			})
			c.failedDiffCounter.Inc(repoLogger)
			continue
		}

		scan := c.scanRepository.Start(quietLogger, "diff-scan", &repo, &fetch)
		c.sniffer.Sniff(
			logger,
			diffscanner.NewDiffScanner(strings.NewReader(diff)),
			func(logger lager.Logger, violation scanners.Violation) error {
				line := violation.Line
				scan.RecordCredential(db.Credential{
					Owner:      repo.Owner,
					Repository: repo.Name,
					Path:       line.Path,
					LineNumber: line.LineNumber,
				})
				return nil
			},
		)

		err = scan.Finish()
		if err != nil {
			repoLogger.Error("failed-to-finish-scan", err)
			c.failedScanCounter.Inc(repoLogger)
		}
	}

	c.successCounter.Inc(repoLogger)

	return nil
}

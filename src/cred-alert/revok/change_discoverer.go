package revok

import (
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
	successCounter       metrics.Counter
	failedCounter        metrics.Counter
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
		successCounter:       emitter.Counter(successMetric),
		failedCounter:        emitter.Counter(failedMetric),
	}
}

func (c *ChangeDiscoverer) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := c.logger.Session("change-discoverer")
	logger.Info("started")

	close(ready)

	timer := c.clock.NewTicker(c.interval)

	defer func() {
		logger.Info("done")
		timer.Stop()
	}()

	c.work(logger)

	for {
		select {
		case <-timer.C():
			c.work(logger)
		case <-signals:
			return nil
		}
	}

	return nil
}

func (c *ChangeDiscoverer) work(logger lager.Logger) error {
	repos, err := c.repositoryRepository.NotFetchedSince(c.clock.Now().Add(-c.interval))
	if err != nil {
		logger.Error("failed-getting-repos", err)
	}

	quietLogger := kolsch.NewLogger()

	for _, repo := range repos {
		repoLogger := logger.WithData(lager.Data{
			"owner":      repo.Owner,
			"repository": repo.Name,
			"path":       repo.Path,
		})

		changes, err := c.gitClient.Fetch(repo.Path)
		if err != nil {
			repoLogger.Error("failed-to-fetch", err)
			continue
		}

		bs, err := json.Marshal(changes)
		if err != nil {
			repoLogger.Error("failed-to-marshal-json", err)
		}

		fetch := db.Fetch{
			Repository: repo,
			Path:       repo.Path,
			Changes:    bs,
		}

		err = c.fetchRepository.SaveFetch(repoLogger, &fetch)
		if err != nil {
			repoLogger.Error("failed-to-save-fetch", err)
			continue
		}

		for _, oids := range changes {
			diff, err := c.gitClient.Diff(repo.Path, oids[0], oids[1])
			if err != nil {
				repoLogger.Error("failed-to-get-diff", err, lager.Data{
					"from": oids[0].String(),
					"to":   oids[1].String(),
				})
				continue
			}

			scan := c.scanRepository.Start(quietLogger, "diff-scan", &repo, &fetch)
			defer finishScan(repoLogger, scan, c.successCounter, c.failedCounter)

			c.sniffer.Sniff(
				quietLogger,
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
		}
	}

	return nil
}

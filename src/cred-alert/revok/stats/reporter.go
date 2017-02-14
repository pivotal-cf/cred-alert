package stats

import (
	"cred-alert/db"
	"cred-alert/metrics"
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"

	"github.com/tedsuo/ifrit"
)

type reporter struct {
	logger lager.Logger
	clock  clock.Clock
	db     db.StatsRepository

	interval time.Duration

	reposGauge         metrics.Gauge
	disabledReposGauge metrics.Gauge
	unclonedReposGauge metrics.Gauge
	fetchGauge         metrics.Gauge
	credentialGauge    metrics.Gauge
}

func NewReporter(
	logger lager.Logger,
	clock clock.Clock,
	interval time.Duration,
	db db.StatsRepository,
	emitter metrics.Emitter,
) ifrit.Runner {
	return &reporter{
		logger: logger,
		clock:  clock,
		db:     db,

		interval: interval,

		reposGauge:         emitter.Gauge("revok.reporter.repo_count"),
		disabledReposGauge: emitter.Gauge("revok.reporter.disabled_repo_count"),
		unclonedReposGauge: emitter.Gauge("revok.reporter.uncloned_repo_count"),
		fetchGauge:         emitter.Gauge("revok.reporter.fetch_count"),
		credentialGauge:    emitter.Gauge("revok.reporter.credential_count"),
	}
}

func (r *reporter) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := r.logger.Session("reporter", lager.Data{
		"interval": r.interval.String(),
	})
	logger.Info("starting")
	defer logger.Info("done")

	ticker := r.clock.NewTicker(r.interval)
	defer ticker.Stop()

	close(ready)

	for {
		select {
		case <-ticker.C():
			r.reportStats(logger)
		case <-signals:
			return nil
		}
	}
}

func (r *reporter) reportStats(logger lager.Logger) {
	reposCount, err := r.db.RepositoryCount()
	if err != nil {
		logger.Error("failed-to-get-repository-count", err)
	} else {
		r.reposGauge.Update(r.logger, float32(reposCount))
	}

	fetchCount, err := r.db.FetchCount()
	if err != nil {
		logger.Error("failed-to-get-fetch-count", err)
	} else {
		r.fetchGauge.Update(r.logger, float32(fetchCount))
	}

	credentialCount, err := r.db.CredentialCount()
	if err != nil {
		logger.Error("failed-to-get-credential-count", err)
	} else {
		r.credentialGauge.Update(r.logger, float32(credentialCount))
	}

	disabledRepoCount, err := r.db.DisabledRepositoryCount()
	if err != nil {
		logger.Error("failed-to-get-disabled-repository-count", err)
	} else {
		r.disabledReposGauge.Update(r.logger, float32(disabledRepoCount))
	}

	unclonedRepoCount, err := r.db.UnclonedRepositoryCount()
	if err != nil {
		logger.Error("failed-to-get-uncloned-repository-count", err)
	} else {
		r.unclonedReposGauge.Update(r.logger, float32(unclonedRepoCount))
	}
}

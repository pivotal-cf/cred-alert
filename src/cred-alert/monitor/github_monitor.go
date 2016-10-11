package monitor

import (
	"cred-alert/metrics"
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
)

const GITHUB_SERVICE_URL = "https://status.github.com/api/status.json"

type githubMonitor struct {
	logger    lager.Logger
	ghService GithubService
	clock     clock.Clock
	interval  time.Duration
	status    metrics.Gauge
}

func NewGithubMonitor(
	logger lager.Logger,
	ghService GithubService,
	clock clock.Clock,
	interval time.Duration,
	emitter metrics.Emitter,
) *githubMonitor {
	return &githubMonitor{
		logger:    logger,
		ghService: ghService,
		clock:     clock,
		interval:  interval,
		status:    emitter.Gauge("services.github_status"),
	}
}

func (g *githubMonitor) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)

	timer := g.clock.NewTicker(g.interval)

	for {
		select {
		case <-timer.C():
			status, err := g.ghService.Status(GITHUB_SERVICE_URL)
			if err != nil {
				g.logger.Error("failed-to-get-github-status", err)
				continue
			}
			g.status.Update(g.logger, float32(status))
		case <-signals:
			return nil
		}
	}
	return nil
}

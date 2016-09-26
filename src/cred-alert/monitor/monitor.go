package monitor

import (
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/ifrit"

	"cred-alert/metrics"
	"cred-alert/revok"
)

type monitor struct {
	logger                lager.Logger
	ghClient              revok.GitHubClient
	clock                 clock.Clock
	interval              time.Duration
	remainingRequestGauge metrics.Gauge
}

func NewMonitor(
	logger lager.Logger,
	ghClient revok.GitHubClient,
	emitter metrics.Emitter,
	clock clock.Clock,
	interval time.Duration,
) ifrit.Runner {
	return &monitor{
		logger: logger.Session("github-monitor", lager.Data{
			"interval": interval.String(),
		}),
		ghClient:              ghClient,
		clock:                 clock,
		interval:              interval,
		remainingRequestGauge: emitter.Gauge("cred_alert.github_remaining_requests"),
	}
}

func (m *monitor) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)

	timer := m.clock.NewTicker(m.interval)

	for {
		select {
		case <-timer.C():
			remaining, err := m.ghClient.RemainingRequests(m.logger)
			if err != nil {
				m.logger.Error("failed-to-get-remaining-requests", err)
				continue
			}
			m.remainingRequestGauge.Update(m.logger, float32(remaining))
		case <-signals:
			return nil
		}
	}

	return nil
}

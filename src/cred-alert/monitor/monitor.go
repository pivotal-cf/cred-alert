package monitor

import (
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/google/go-github/github"
	"github.com/tedsuo/ifrit"

	"cred-alert/metrics"
)

//go:generate counterfeiter . RateClient

type RateClient interface {
	RateLimits() (*github.RateLimits, *github.Response, error)
}

type monitor struct {
	logger                lager.Logger
	ghClient              RateClient
	clock                 clock.Clock
	interval              time.Duration
	remainingRequestGauge metrics.Gauge
}

func NewMonitor(
	logger lager.Logger,
	ghClient RateClient,
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
			rates, _, err := m.ghClient.RateLimits()
			if err != nil {
				m.logger.Error("failed-to-get-remaining-requests", err)
				continue
			}
			m.remainingRequestGauge.Update(m.logger, float32(rates.Core.Remaining))
		case <-signals:
			return nil
		}
	}
}

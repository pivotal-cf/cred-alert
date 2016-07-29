package metrics

import (
	"time"

	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . Timer

type Timer interface {
	Time(lager.Logger, func(), ...string)
}

type timer struct {
	metric Metric
}

func NewTimer(metric Metric) *timer {
	return &timer{
		metric: metric,
	}
}

func (t *timer) Time(logger lager.Logger, work func(), tags ...string) {
	logger = logger.Session("time")
	logger.Debug("starting")
	startTime := time.Now()

	work()

	duration := time.Since(startTime)

	logger.Debug("done", lager.Data{
		"duration": duration.String(),
	})

	t.metric.Update(logger, float32(duration.Seconds()), tags...)
}

type nullTimer struct{}

func (t *nullTimer) Time(logger lager.Logger, fn func(), tags ...string) {
	logger.Session("time").Debug("done")
	fn()
}

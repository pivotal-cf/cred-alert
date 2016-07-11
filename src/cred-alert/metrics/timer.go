package metrics

import (
	"time"

	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . Timer

type Timer interface {
	Time(lager.Logger, func())
}

type timer struct {
	metric Metric
}

func NewTimer(metric Metric) *timer {
	return &timer{
		metric: metric,
	}
}

func (t *timer) Time(logger lager.Logger, fn func()) {
	startTime := time.Now()

	fn()
	duration := time.Since(startTime)

	logger.Debug("stopping-timer", lager.Data{
		"duration": duration.String(),
	})
	t.metric.Update(logger, float32(duration.Seconds()))
}

func (t *timer) Start(logger lager.Logger) {
	startTime := time.Now()
	logger.Debug("starting-timer", lager.Data{
		"start-time": startTime.String(),
	})
}

type nullTimer struct{}

func (t *nullTimer) Time(logger lager.Logger, fn func()) {
	fn()
}

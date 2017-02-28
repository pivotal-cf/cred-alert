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
	gauge Gauge
}

func NewTimer(gauge Gauge) *timer {
	return &timer{
		gauge: gauge,
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

	t.gauge.Update(logger, float32(duration.Seconds()), tags...)
}

type nullTimer struct{}

func (t *nullTimer) Time(logger lager.Logger, fn func(), tags ...string) {
	logger.Session("time").Debug("done")
	fn()
}

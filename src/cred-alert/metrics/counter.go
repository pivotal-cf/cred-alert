package metrics

import "code.cloudfoundry.org/lager"

//go:generate counterfeiter . Counter

type Counter interface {
	Inc(lager.Logger, ...string)
	IncN(lager.Logger, int, ...string)
}

type counter struct {
	gauge Gauge
}

func NewCounter(gauge Gauge) *counter {
	return &counter{
		gauge: gauge,
	}
}

func (c *counter) Inc(logger lager.Logger, tags ...string) {
	c.IncN(logger, 1, tags...)
}

func (c *counter) IncN(logger lager.Logger, count int, tags ...string) {
	if count <= 0 {
		return
	}
	c.gauge.Update(logger, float32(count), tags...)
}

func NewNullCounter(gauge Gauge) *nullCounter {
	return &nullCounter{
		gauge: gauge,
	}
}

type nullCounter struct {
	gauge Gauge
}

func (c *nullCounter) Inc(logger lager.Logger, tags ...string) {
	c.IncN(logger, 1, tags...)
}

func (c *nullCounter) IncN(logger lager.Logger, count int, tags ...string) {
	c.gauge.Update(logger, float32(count), tags...)
}

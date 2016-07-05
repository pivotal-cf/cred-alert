package metrics

import "github.com/pivotal-golang/lager"

//go:generate counterfeiter . Counter

type Counter interface {
	Inc(lager.Logger)
	IncN(lager.Logger, int)
}

type counter struct {
	metric Metric
}

func NewCounter(metric Metric) *counter {
	return &counter{
		metric: metric,
	}
}

func (c *counter) Inc(logger lager.Logger) {
	c.IncN(logger, 1)
}

func (c *counter) IncN(logger lager.Logger, count int) {
	if count <= 0 {
		return
	}
	c.metric.Update(logger, float32(count))
}

func NewNullCounter(metric Metric) *nullCounter {
	return &nullCounter{
		metric: metric,
	}
}

type nullCounter struct {
	metric Metric
}

func (c *nullCounter) Inc(logger lager.Logger) {
	c.IncN(logger, 1)
}

func (c *nullCounter) IncN(logger lager.Logger, count int) {
	c.metric.Update(logger, float32(count))
}

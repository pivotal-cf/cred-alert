package metrics

import (
	"cred-alert/datadog"

	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . Counter

type Counter interface {
	Inc(lager.Logger)
	IncN(lager.Logger, int)
}

type counter struct {
	name    string
	emitter *emitter
}

func (c *counter) Inc(logger lager.Logger) {
	c.IncN(logger, 1)
}

func (c *counter) IncN(logger lager.Logger, count int) {
	logger = logger.Session("emit-count", lager.Data{
		"name":        c.name,
		"environment": c.emitter.environment,
		"increment":   count,
	})

	if count <= 0 {
		return
	}

	metric := c.emitter.client.BuildCountMetric(c.name, float32(count), c.emitter.environment)
	err := c.emitter.client.PublishSeries([]datadog.Metric{metric})
	if err != nil {
		logger.Error("failed", err)
		return
	}

	logger.Debug("emitted")
}

type nullCounter struct {
	name        string
	environment string
}

func (c *nullCounter) Inc(logger lager.Logger) {
	c.IncN(logger, 1)
}

func (c *nullCounter) IncN(logger lager.Logger, count int) {
	logger.Session("emit-count", lager.Data{
		"name":        c.name,
		"environment": c.environment,
		"increment":   count,
	}).Debug("emitted")
}

package logging

import (
	"github.com/pivotal-golang/lager"

	"cred-alert/datadog"
)

//go:generate counterfeiter . Emitter

type Emitter interface {
	Counter(name string) Counter
}

//go:generate counterfeiter . Counter

type Counter interface {
	Inc(lager.Logger)
	IncN(lager.Logger, int)
}

func BuildEmitter(apiKey string, environment string) Emitter {
	if apiKey == "" {
		return &nullEmitter{
			environment: environment,
		}
	}

	client := datadog.NewClient(apiKey)

	return NewEmitter(client, environment)
}

func NewEmitter(dataDogClient datadog.Client, environment string) Emitter {
	return &emitter{
		client:      dataDogClient,
		environment: environment,
	}
}

type emitter struct {
	client      datadog.Client
	environment string
}

func (emitter *emitter) Counter(name string) Counter {
	return &counter{
		name:    name,
		emitter: emitter,
	}
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

type nullEmitter struct {
	name        string
	environment string
}

func (e *nullEmitter) Counter(name string) Counter {
	return &nullCounter{
		name:        e.name,
		environment: e.environment,
	}
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

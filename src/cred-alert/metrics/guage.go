package metrics

import (
	"cred-alert/datadog"

	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . Guage

type Guage interface {
	Update(lager.Logger, float32)
}

type guage struct {
	name    string
	emitter *emitter
}

func (g *guage) Update(logger lager.Logger, value float32) {
	logger = logger.Session("emit-guage", lager.Data{
		"name":        g.name,
		"environment": g.emitter.environment,
		"value":       value,
	})

	metric := g.emitter.client.BuildMetric(datadog.GUAGE_METRIC_TYPE, g.name, value, g.emitter.environment)
	err := g.emitter.client.PublishSeries([]datadog.Metric{metric})
	if err != nil {
		logger.Error("failed", err)
	}

	logger.Debug("emitted")
}

type nullGuage struct {
	name        string
	environment string
}

func (g *nullGuage) Update(logger lager.Logger, value float32) {
	logger.Session("emit-guage-update", lager.Data{
		"name":        g.name,
		"environment": g.environment,
		"value":       value,
	}).Debug("emitted")
}

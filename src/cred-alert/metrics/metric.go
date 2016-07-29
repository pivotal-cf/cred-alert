package metrics

import (
	"cred-alert/datadog"

	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . Metric

type Metric interface {
	Update(lager.Logger, float32, ...string)
}

type metric struct {
	name       string
	metricType string
	emitter    *emitter
}

func NewMetric(name string, metricType string, emitter *emitter) *metric {
	return &metric{
		name:       name,
		metricType: metricType,
		emitter:    emitter,
	}
}

func (m *metric) Update(logger lager.Logger, value float32, tags ...string) {
	logger = logger.Session("update", lager.Data{
		"name":        m.name,
		"type":        m.metricType,
		"environment": m.emitter.environment,
		"value":       value,
	})
	logger.Debug("starting")

	tagsWithEnv := append(tags, m.emitter.environment)
	ddMetric := m.emitter.client.BuildMetric(m.metricType, m.name, value, tagsWithEnv...)
	err := m.emitter.client.PublishSeries([]datadog.Metric{ddMetric})
	if err != nil {
		logger.Error("failed", err)
	}

	logger.Debug("done")
}

type nullMetric struct {
	name        string
	metricType  string
	environment string
}

func (m *nullMetric) Update(logger lager.Logger, value float32, tags ...string) {
	logger.Session("update", lager.Data{
		"name":        m.name,
		"type":        m.metricType,
		"environment": m.environment,
		"value":       value,
		"tags":        tags,
	}).Debug("done")
}

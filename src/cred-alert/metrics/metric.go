package metrics

import (
	"cred-alert/datadog"

	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . Metric

type Metric interface {
	Update(lager.Logger, float32)
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

func (m *metric) Update(logger lager.Logger, value float32) {
	logger = logger.Session("emit-metric", lager.Data{
		"name":        m.name,
		"type":        m.metricType,
		"environment": m.emitter.environment,
		"value":       value,
	})

	ddMetric := m.emitter.client.BuildMetric(m.metricType, m.name, value, m.emitter.environment)

	err := m.emitter.client.PublishSeries([]datadog.Metric{ddMetric})
	if err != nil {
		logger.Error("failed", err)
	}

	logger.Debug("emitted")
}

type nullMetric struct {
	name        string
	metricType  string
	environment string
}

func (m *nullMetric) Update(logger lager.Logger, value float32) {
	logger.Session("emit-metric", lager.Data{
		"name":        m.name,
		"type":        m.metricType,
		"environment": m.environment,
		"value":       value,
	}).Debug("emitted")
}

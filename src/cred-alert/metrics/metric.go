package metrics

import (
	"cred-alert/datadog"

	"code.cloudfoundry.org/lager"
)

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

	tagsWithEnv := append(tags, m.emitter.environment)
	ddMetric := m.emitter.client.BuildMetric(m.metricType, m.name, value, tagsWithEnv...)
	go m.emitter.client.PublishSeries(logger, []datadog.Metric{ddMetric})
}

type nullMetric struct {
	name       string
	metricType string
}

func (m *nullMetric) Update(logger lager.Logger, value float32, tags ...string) {}

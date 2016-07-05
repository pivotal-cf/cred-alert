package metrics

import "cred-alert/datadog"

//go:generate counterfeiter . Emitter

type Emitter interface {
	Counter(name string) Counter
	Guage(name string) Guage
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

func NewEmitter(dataDogClient datadog.Client, environment string) *emitter {
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
	metric := &metric{
		name:       name,
		metricType: datadog.COUNTER_METRIC_TYPE,
		emitter:    emitter,
	}

	return NewCounter(metric)
}

func (emitter *emitter) Guage(name string) Guage {
	return &metric{
		name:       name,
		metricType: datadog.GUAGE_METRIC_TYPE,
		emitter:    emitter,
	}
}

type nullEmitter struct {
	name        string
	environment string
}

func (e *nullEmitter) Counter(name string) Counter {
	metric := &nullMetric{}
	return &nullCounter{
		metric: metric,
	}
}

func (e *nullEmitter) Guage(name string) Guage {
	return &nullMetric{
		name:       name,
		metricType: datadog.GUAGE_METRIC_TYPE,
	}
}

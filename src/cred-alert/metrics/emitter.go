package metrics

import "cred-alert/datadog"

//go:generate counterfeiter . Emitter

type Emitter interface {
	Counter(name string) Counter
	Gauge(name string) Gauge
	Timer(name string) Timer
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
		metricType: datadog.CounterMetricType,
		emitter:    emitter,
	}

	return NewCounter(metric)
}

func (emitter *emitter) Gauge(name string) Gauge {
	return &metric{
		name:       name,
		metricType: datadog.GaugeMetricType,
		emitter:    emitter,
	}
}

func (emitter *emitter) Timer(name string) Timer {
	metric := &metric{
		name:    name,
		emitter: emitter,
	}

	return NewTimer(metric)
}

type nullEmitter struct {
	environment string
}

func (e *nullEmitter) Counter(name string) Counter {
	metric := &nullMetric{}
	return &nullCounter{
		metric: metric,
	}
}

func (e *nullEmitter) Gauge(name string) Gauge {
	return &nullMetric{
		name:       name,
		metricType: datadog.GaugeMetricType,
	}
}

func (e *nullEmitter) Timer(name string) Timer {
	return &nullTimer{}
}

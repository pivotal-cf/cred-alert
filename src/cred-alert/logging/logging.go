package logging

import (
	"time"

	"github.com/pivotal-golang/lager"

	"cred-alert/datadog"
)

func BuildEmitter(apiKey string, environment string) Emitter {
	if apiKey == "" {
		return &nullEmitter{
			environment: environment,
		}
	}

	client := datadog.NewClient(apiKey)

	return &emitter{
		dataDogClient: client,
		environment:   environment,
	}
}

//go:generate counterfeiter . Emitter

type Emitter interface {
	CountViolation(logger lager.Logger, count int)
}

type nullEmitter struct {
	environment string
}

func (e *nullEmitter) CountViolation(logger lager.Logger, count int) {
	logger.Session("emit-violation-count", lager.Data{
		"environment":     e.environment,
		"violation-count": count,
	}).Debug("emitted")
}

type emitter struct {
	dataDogClient datadog.Client
	environment   string
}

func (e *emitter) CountViolation(logger lager.Logger, count int) {
	logger = logger.Session("emit-violation-count", lager.Data{
		"environment":     e.environment,
		"violation-count": count,
	})

	if count <= 0 {
		return
	}

	points := []datadog.Point{}
	tags := []string{e.environment}

	metric := datadog.Metric{
		Name:   "cred_alert.violations",
		Points: append(points, datadog.Point{time.Now(), float32(count)}),
		Type:   "count",
		Tags:   tags,
	}
	series := []datadog.Metric{}

	client := e.dataDogClient
	err := client.PublishSeries(append(series, metric))
	if err != nil {
		logger.Error("failed", err)
		return
	}

	logger.Debug("emitted")
}

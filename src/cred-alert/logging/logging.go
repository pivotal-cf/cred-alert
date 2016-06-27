package logging

import (
	"time"

	"github.com/pivotal-golang/lager"

	"cred-alert/datadog"
)

func NewEmitter(dataDogClient datadog.Client, environment string) Emitter {
	return &emitter{
		client:      dataDogClient,
		environment: environment,
	}
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

//go:generate counterfeiter . Emitter

type Emitter interface {
	CountViolation(logger lager.Logger, count int)
	CountAPIRequest(logger lager.Logger)
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

func (e *nullEmitter) CountAPIRequest(logger lager.Logger) {
	logger.Session("emit-api-request-count", lager.Data{
		"environment": e.environment,
	}).Debug("emitted")
}

type emitter struct {
	client      datadog.Client
	environment string
}

func (e *emitter) CountViolation(logger lager.Logger, count int) {
	logger = logger.Session("emit-violation-count", lager.Data{
		"environment":     e.environment,
		"violation-count": count,
	})

	if count <= 0 {
		return
	}

	metric := datadog.Metric{
		Name: "cred_alert.violations",
		Points: []datadog.Point{
			{Timestamp: time.Now(), Value: float32(count)},
		},
		Type: "count",
		Tags: []string{e.environment},
	}

	err := e.client.PublishSeries([]datadog.Metric{metric})
	if err != nil {
		logger.Error("failed", err)
		return
	}

	logger.Debug("emitted")
}

func (e *emitter) CountAPIRequest(logger lager.Logger) {
	logger = logger.Session("emit-api-request-count", lager.Data{
		"environment": e.environment,
	})

	metric := datadog.Metric{
		Name: "cred_alert.webhook_requests",
		Points: []datadog.Point{
			{Timestamp: time.Now(), Value: float32(1)},
		},
		Type: "count",
		Tags: []string{e.environment},
	}

	err := e.client.PublishSeries([]datadog.Metric{metric})
	if err != nil {
		logger.Error("failed", err)
		return
	}

	logger.Debug("emitted")
}

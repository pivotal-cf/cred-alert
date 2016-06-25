package logging

import (
	"cred-alert/datadog"
	"errors"

	"github.com/pivotal-golang/lager"

	"fmt"
	"os"
	"time"
)

type Emitter interface {
	CountViolation(logger lager.Logger, count int)
}

type emitter struct {
	dataDogClient  datadog.Client
	environmentTag string
}

func NewEmitter(client datadog.Client) *emitter {
	return &emitter{dataDogClient: client}
}

func DefaultEmitter() (Emitter, error) {
	apiKey := os.Getenv("DATA_DOG_API_KEY")
	environmentTag := os.Getenv("DATA_DOG_ENVIRONMENT_TAG")

	if apiKey == "" {
		return nil, errors.New("Error: environment variable DATA_DOG_API_KEY not set")
	}

	client := datadog.NewClient(apiKey)
	emitter := NewEmitter(client)

	if environmentTag == "" {
		fmt.Printf("Warning: DATA_DOG_ENVIRONMENT_TAG not set")
	} else {
		emitter.environmentTag = environmentTag
	}

	return emitter, nil
}

func (e *emitter) CountViolation(logger lager.Logger, count int) {
	logger.Session("emit-violation-count", lager.Data{
		"violation-count": count,
	})

	if count <= 0 {
		return
	}

	points := []datadog.Point{}
	tags := []string{"credential_violation"}
	if e.environmentTag != "" {
		tags = append(tags, e.environmentTag)
	}
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

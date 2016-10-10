package services

import (
	"cred-alert/metrics"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/ifrit"
)

type githubResponseBody struct {
	status       string
	last_updated string
}

//go:generate counterfeiter . GithubServiceClient

type GithubServiceClient interface {
	GithubStatus(string) (int, error)
}

type githubService struct {
	logger          lager.Logger
	clock           clock.Clock
	ghServiceClient GithubServiceClient
	interval        time.Duration
	status          metrics.Gauge
}

const GITHUB_SERVICE_URL = "https://status.github.com/api/status.json"

func NewGithubService(
	logger lager.Logger,
	ghServiceClient GithubServiceClient,
	emitter metrics.Emitter,
	clock clock.Clock,
	interval time.Duration,
) ifrit.Runner {
	return &githubService{
		logger: logger.Session("github-service", lager.Data{
			"interval": interval.String(),
		}),
		ghServiceClient: ghServiceClient,
		clock:           clock,
		interval:        interval,
		status:          emitter.Gauge("services.github_status"),
	}
}

func (g *githubService) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)

	timer := g.clock.NewTicker(g.interval)

	for {
		select {
		case <-timer.C():
			statusCode, err := g.ghServiceClient.GithubStatus(GITHUB_SERVICE_URL)
			if err != nil {
				g.logger.Error("failed-to-get-github-status", err)
				continue
			}
			g.status.Update(g.logger, float32(statusCode))
		case <-signals:
			return nil
		}
	}
	return nil
}

func GithubStatus(serverURL string) (int, error) {
	client := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}
	req, err := http.NewRequest("GET", serverURL, nil)
	if err != nil {
		return 1, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return 1, err
	}

	var gh map[string]string

	content, _ := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal([]byte(content), &gh)
	if err != nil {
		return 1, err
	}

	if gh["status"] == "good" {
		return 0, nil
	}

	return 1, nil
}

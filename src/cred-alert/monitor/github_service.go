package monitor

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . GithubService

type GithubService interface {
	Status(string) (int, error)
}

type githubService struct {
	logger lager.Logger
}

func NewGithubService(
	logger lager.Logger,
) GithubService {
	return &githubService{
		logger: logger.Session("github-service"),
	}
}

func (g *githubService) Status(serverURL string) (int, error) {
	client := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}
	req, err := http.NewRequest("GET", serverURL, nil)
	if err != nil {
		g.logger.Error("cannot-create-http-request", err)
		return 1, err
	}

	resp, err := client.Do(req)
	if err != nil {
		g.logger.Error("github-request-error", err)
		return 1, err
	}

	var gh map[string]string

	content, _ := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal([]byte(content), &gh)
	if err != nil {
		g.logger.Error("github-response-error", err)
		return 1, err
	}

	if gh["status"] == "good" {
		return 0, nil
	}

	return 1, nil
}

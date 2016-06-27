package github

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/cloudfoundry/gunk/urljoiner"
	"github.com/pivotal-golang/lager"
)

const DEFAULT_GITHUB_URL = "https://api.github.com/"

//go:generate counterfeiter . Client

type Client interface {
	CompareRefs(logger lager.Logger, owner, repo, base, head string) (string, error)
}

type client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string, httpClient *http.Client) *client {
	return &client{
		baseURL:    baseURL,
		httpClient: httpClient,
	}
}

func (c *client) CompareRefs(logger lager.Logger, owner, repo, base, head string) (string, error) {
	logger = logger.Session("comparing-refs")

	url := urljoiner.Join(c.baseURL, "repos", owner, repo, "compare", base+"..."+head)
	request, _ := http.NewRequest("GET", url, nil)
	request.Header.Set("Accept", "application/vnd.github.diff")

	response, err := c.httpClient.Do(request)
	if err != nil {
		logger.Error("failed", err)
		return "", err
	}

	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		logger.Error("failed", err)
		return "", err
	}

	if response.StatusCode != http.StatusOK {
		err := errors.New("status code not 200")
		logger.Error("unexpected-status-code", err, lager.Data{
			"status": fmt.Sprintf("%s (%d)", http.StatusText(response.StatusCode), response.StatusCode),
			"body":   string(body),
		})

		return "", err
	}

	return string(body), nil
}

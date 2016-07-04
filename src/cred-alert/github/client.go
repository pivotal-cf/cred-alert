package github

import (
	"cred-alert/metrics"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/cloudfoundry/gunk/urljoiner"
	"github.com/google/go-github/github"
	"github.com/pivotal-golang/lager"
)

const DEFAULT_GITHUB_URL = "https://api.github.com/"

//go:generate counterfeiter . Client

type Client interface {
	CompareRefs(logger lager.Logger, owner, repo, base, head string) (string, error)
}

type client struct {
	baseURL        string
	httpClient     *http.Client
	rateLimitGuage metrics.Guage
}

func NewClient(baseURL string, httpClient *http.Client, emitter metrics.Emitter) *client {
	return &client{
		baseURL:        baseURL,
		httpClient:     httpClient,
		rateLimitGuage: emitter.Guage("cred_alert.github_remaining_requests"),
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

	ratelimit := c.rateFromResponse(logger, response)
	c.rateLimitGuage.Update(logger, float32(ratelimit.Remaining))
	return string(body), nil
}

func (c *client) rateFromResponse(logger lager.Logger, response *http.Response) github.Rate {
	header := response.Header
	reset, err := strconv.ParseInt(header["X-Ratelimit-Reset"][0], 10, 64)
	if err != nil {
		logger.Error("Error getting rate limit form header", err)
	}

	timestamp := github.Timestamp{Time: time.Unix(reset, 0)}

	remain, err := strconv.Atoi(header["X-Ratelimit-Remaining"][0])
	if err != nil {
		logger.Error("Error getting rate limit from header.", err)
	}

	return github.Rate{
		Remaining: remain,
		Reset:     timestamp,
	}
}

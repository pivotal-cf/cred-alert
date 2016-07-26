package github

import (
	"cred-alert/metrics"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/cloudfoundry/gunk/urljoiner"
	"github.com/google/go-github/github"
	"github.com/pivotal-golang/lager"
)

const DefaultGitHubURL = "https://api.github.com/"

//go:generate counterfeiter . Client

type Client interface {
	CompareRefs(logger lager.Logger, owner, repo, base, head string) (string, error)
	ArchiveLink(owner, repo, ref string) (*url.URL, error)
	Parents(logger lager.Logger, owner, repo, sha string) ([]string, error)
}

type client struct {
	baseURL        string
	httpClient     *http.Client
	rateLimitGauge metrics.Gauge
}

func NewClient(baseURL string, httpClient *http.Client, emitter metrics.Emitter) *client {
	return &client{
		baseURL:        baseURL,
		httpClient:     httpClient,
		rateLimitGauge: emitter.Gauge("cred_alert.github_remaining_requests"),
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

	if ratelimit, err := c.rateFromResponse(logger, response); err == nil {
		c.rateLimitGauge.Update(logger, float32(ratelimit.Remaining))
	}
	return string(body), nil
}

func (c *client) rateFromResponse(logger lager.Logger, response *http.Response) (github.Rate, error) {
	header := response.Header
	reset, err := strconv.ParseInt(header.Get("X-Ratelimit-Reset"), 10, 64)
	if err != nil {
		logger.Error("Error getting rate limit form header", err)
		return github.Rate{}, err
	}

	timestamp := github.Timestamp{Time: time.Unix(reset, 0)}

	remain, err := strconv.Atoi(header.Get("X-Ratelimit-Remaining"))
	if err != nil {
		logger.Error("Error getting rate limit from header.", err)
		return github.Rate{}, err
	}

	return github.Rate{
		Remaining: remain,
		Reset:     timestamp,
	}, nil
}

func (c *client) ArchiveLink(owner, repo string, ref string) (*url.URL, error) {
	return url.Parse(urljoiner.Join(c.baseURL, "repos", owner, repo, "zipball", ref))
}

func (c *client) Parents(logger lager.Logger, owner, repo, sha string) ([]string, error) {
	logger = logger.Session("fetching-parents", lager.Data{
		"Owner": owner,
		"Repo":  repo,
		"SHA":   sha,
	})

	url := urljoiner.Join(c.baseURL, "repos", owner, repo, "commits", sha)
	request, _ := http.NewRequest("GET", url, nil)
	response, err := c.httpClient.Do(request)
	if err != nil {
		logger.Error("failed", err)
		return []string{}, err
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		logger.Error("read-error", err)
		return []string{}, err
	}

	if response.StatusCode != http.StatusOK {
		err := errors.New("status code not 200")
		logger.Error("unexpected-status-code", err, lager.Data{
			"status": fmt.Sprintf("%s (%d)", http.StatusText(response.StatusCode), response.StatusCode),
			"body":   string(body),
		})
		return []string{}, err
	}

	type Parent struct {
		Sha string
	}

	type Commit struct {
		Parents []Parent
	}

	var commit Commit
	if err := json.Unmarshal(body, &commit); err != nil {
		logger.Error("failed", err)
		return []string{}, err
	}

	parentShas := []string{}
	for _, parent := range commit.Parents {
		parentShas = append(parentShas, parent.Sha)
	}

	if ratelimit, err := c.rateFromResponse(logger, response); err == nil {
		c.rateLimitGauge.Update(logger, float32(ratelimit.Remaining))
	}

	return parentShas, nil
}

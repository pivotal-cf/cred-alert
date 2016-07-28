package githubclient

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
	"code.cloudfoundry.org/lager"
)

const DefaultGitHubURL = "https://api.github.com/"

//go:generate counterfeiter . Client

type Client interface {
	CompareRefs(logger lager.Logger, owner, repo, base, head string) (string, error)
	ArchiveLink(owner, repo, ref string) (*url.URL, error)
	CommitInfo(logger lager.Logger, owner, repo, sha string) (CommitInfo, error)
}

type CommitInfo struct {
	Message string
	Parents []string
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
	logger = logger.Session("compare-refs")
	logger.Info("starting")

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

		logger.Error("failed", err)
		return "", err
	}

	if ratelimit, err := c.rateFromResponse(logger, response); err == nil {
		c.rateLimitGauge.Update(logger, float32(ratelimit.Remaining))
	}

	logger.Info("done")
	return string(body), nil
}

func (c *client) rateFromResponse(logger lager.Logger, response *http.Response) (github.Rate, error) {
	logger = logger.Session("rate-from-response")
	logger.Info("starting")

	header := response.Header
	reset, err := strconv.ParseInt(header.Get("X-Ratelimit-Reset"), 10, 64)
	if err != nil {
		logger.Error("failed", err)
		return github.Rate{}, err
	}

	timestamp := github.Timestamp{Time: time.Unix(reset, 0)}

	remain, err := strconv.Atoi(header.Get("X-Ratelimit-Remaining"))
	if err != nil {
		logger.Error("failed", err)
		return github.Rate{}, err
	}

	logger.Info("done")
	return github.Rate{
		Remaining: remain,
		Reset:     timestamp,
	}, nil
}

func (c *client) ArchiveLink(owner, repo string, ref string) (*url.URL, error) {
	return url.Parse(urljoiner.Join(c.baseURL, "repos", owner, repo, "zipball", ref))
}

type commit struct {
	Message string `json:"message"`
}

type parent struct {
	SHA string `json:"sha"`
}

var commitResponse struct {
	Commit  commit   `json:"commit"`
	Parents []parent `json:"parents"`
}

func (c *client) CommitInfo(logger lager.Logger, owner, repo, sha string) (CommitInfo, error) {
	logger = logger.Session("commit-info", lager.Data{
		"Owner": owner,
		"Repo":  repo,
		"SHA":   sha,
	})
	logger.Info("starting")

	url := urljoiner.Join(c.baseURL, "repos", owner, repo, "commits", sha)
	request, _ := http.NewRequest("GET", url, nil)
	response, err := c.httpClient.Do(request)
	if err != nil {
		logger.Error("failed", err)
		return CommitInfo{}, err
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		logger.Error("failed", err)
		return CommitInfo{}, err
	}

	if response.StatusCode != http.StatusOK {
		err := fmt.Errorf("bad response (!200): %d", response.StatusCode)
		logger.Error("failed", err, lager.Data{
			"status": fmt.Sprintf("%s (%d)", http.StatusText(response.StatusCode), response.StatusCode),
			"body":   string(body),
		})
		return CommitInfo{}, err
	}

	if err := json.Unmarshal(body, &commitResponse); err != nil {
		logger.Error("failed", err)
		return CommitInfo{}, err
	}

	parentShas := []string{}
	for _, parent := range commitResponse.Parents {
		parentShas = append(parentShas, parent.SHA)
	}

	if ratelimit, err := c.rateFromResponse(logger, response); err == nil {
		c.rateLimitGauge.Update(logger, float32(ratelimit.Remaining))
	}

	logger.Info("done")
	return CommitInfo{
		Message: commitResponse.Commit.Message,
		Parents: parentShas,
	}, nil
}

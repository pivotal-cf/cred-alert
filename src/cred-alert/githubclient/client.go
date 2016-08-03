package githubclient

import (
	"cred-alert/metrics"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry/gunk/urljoiner"
	"github.com/google/go-github/github"
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
	logger.Debug("starting")

	url := urljoiner.Join(c.baseURL, "repos", owner, repo, "compare", base+"..."+head)

	body, err := c.responseBodyFrom(logger, url, map[string]string{"Accept": "application/vnd.github.diff"})
	if err != nil {
		logger.Error("failed", err)
		return "", err
	}

	logger.Debug("done")
	return string(body), nil
}

func (c *client) ArchiveLink(owner, repo string, ref string) (*url.URL, error) {
	reqUrl := urljoiner.Join(c.baseURL, "repos", owner, repo, "zipball", ref)

	req, err := http.NewRequest("GET", reqUrl, nil)
	if err != nil {
		return nil, err
	}

	var resp *http.Response
	resp, err = c.httpClient.Transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusFound {
		return nil, fmt.Errorf("Unexpected response status code: %d", resp.StatusCode)
	}

	return url.Parse(resp.Header.Get("Location"))
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
	logger.Debug("starting")

	url := urljoiner.Join(c.baseURL, "repos", owner, repo, "commits", sha)

	body, err := c.responseBodyFrom(logger, url, map[string]string{})
	if err != nil {
		logger.Error("failed", err)
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

	logger.Debug("done")
	return CommitInfo{
		Message: commitResponse.Commit.Message,
		Parents: parentShas,
	}, nil
}

func (c *client) rateFromResponse(logger lager.Logger, response *http.Response) (github.Rate, error) {
	logger = logger.Session("rate-from-response")
	logger.Debug("starting")

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

	logger.Debug("done")
	return github.Rate{
		Remaining: remain,
		Reset:     timestamp,
	}, nil
}

func (c *client) responseBodyFrom(logger lager.Logger, url string, headers map[string]string) ([]byte, error) {
	logger = logger.Session("response-body-from")
	logger.Info("starting", lager.Data{"url": url})

	request, _ := http.NewRequest("GET", url, nil)

	for headerName, headerValue := range headers {
		request.Header.Set(headerName, headerValue)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		logger.Error("failed", err)
		return []byte{}, err
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		logger.Error("failed", err)
		return []byte{}, err
	}

	if response.StatusCode != http.StatusOK {
		err := fmt.Errorf("bad response (!200): %d", response.StatusCode)
		logger.Error("failed", err, lager.Data{
			"status": fmt.Sprintf("%s (%d)", http.StatusText(response.StatusCode), response.StatusCode),
			"body":   body,
		})
		return []byte{}, err
	}

	if ratelimit, err := c.rateFromResponse(logger, response); err == nil {
		c.rateLimitGauge.Update(logger, float32(ratelimit.Remaining))
	}

	logger.Debug("done")
	return body, nil
}

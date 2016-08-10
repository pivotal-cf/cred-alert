package githubclient

import (
	"bytes"
	"cred-alert/metrics"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"

	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry/gunk/urljoiner"
)

const DefaultGitHubURL = "https://api.github.com/"
const ErrNotFound = Error("githubclient-not-found")

type Error string

func (e Error) Error() string { return string(e) }

//go:generate counterfeiter . Client

type Client interface {
	CompareRefs(logger lager.Logger, owner, repo, base, head string) (io.Reader, error)
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

func (c *client) CompareRefs(logger lager.Logger, owner, repo, base, head string) (io.Reader, error) {
	logger = logger.Session("compare-refs")
	logger.Debug("starting")

	url := urljoiner.Join(c.baseURL, "repos", owner, repo, "compare", base+"..."+head)

	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logger.Error("failed", err)
		return nil, err
	}
	request.Header.Set("Accept", "application/vnd.github.diff")

	response, err := c.doRequest(logger, request)
	if err != nil {
		logger.Error("failed", err)
		return nil, err
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		logger.Error("failed", err)
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		err := fmt.Errorf("bad response (!200): %d", response.StatusCode)
		logger.Error("failed", err, lager.Data{
			"status": fmt.Sprintf("%s (%d)", http.StatusText(response.StatusCode), response.StatusCode),
			"body":   body,
		})
		return nil, err
	}

	logger.Debug("done")
	return bytes.NewReader(body), nil
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
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
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

	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logger.Error("failed", err)
		return CommitInfo{}, err
	}

	response, err := c.doRequest(logger, request)
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
			"body":   body,
		})

		if response.StatusCode == http.StatusNotFound {
			return CommitInfo{}, ErrNotFound
		} else {
			return CommitInfo{}, err
		}
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

func (c *client) doRequest(logger lager.Logger, request *http.Request) (*http.Response, error) {
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, err
	}

	remain, err := strconv.Atoi(response.Header.Get("X-RateLimit-Remaining"))
	if err == nil {
		c.rateLimitGauge.Update(logger, float32(remain))
	}

	return response, nil
}

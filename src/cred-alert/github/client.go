package github

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/cloudfoundry/gunk/urljoiner"
)

type Client interface {
	CompareRefs(owner, repo, base, head string) (string, error)
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

func (c *client) CompareRefs(owner, repo, base, head string) (string, error) {
	url := urljoiner.Join(c.baseURL, "repos", owner, repo, "compare", base+"..."+head)

	request, _ := http.NewRequest("GET", url, nil)
	request.Header.Set("Accept", "application/vnd.github.diff")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", err
	}

	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status code (%d): %s", response.StatusCode, string(body))
	}

	return string(body), nil
}

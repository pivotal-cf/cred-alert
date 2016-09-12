package revok

import (
	"encoding/json"

	"code.cloudfoundry.org/lager"
	"github.com/google/go-github/github"
)

//go:generate counterfeiter . GitHubClient

type GitHubClient interface {
	ListRepositories(lager.Logger) ([]GitHubRepository, error)
}

type client struct {
	ghClient *github.Client
}

func NewGitHubClient(ghClient *github.Client) GitHubClient {
	return &client{
		ghClient: ghClient,
	}
}

func (c *client) ListRepositories(logger lager.Logger) ([]GitHubRepository, error) {
	logger = logger.Session("list-originalRepositories")

	opts := &github.RepositoryListOptions{
		ListOptions: github.ListOptions{PerPage: 30},
	}

	var originalRepos []*github.Repository

	for {
		rs, resp, err := c.ghClient.Repositories.List("", opts)
		if err != nil {
			logger.Error("failed", err, lager.Data{
				"fetching-page": opts.ListOptions.Page,
			})
			return nil, err
		}

		originalRepos = append(originalRepos, rs...)

		if resp.NextPage == 0 {
			break
		}

		opts.ListOptions.Page = resp.NextPage
	}

	var repos []GitHubRepository

	for _, r := range originalRepos {
		rawJSONBytes, err := json.Marshal(r)
		if err != nil {
			logger.Error("failed-to-marshal-json", err)
			return nil, err
		}

		repos = append(repos, GitHubRepository{
			Name:          *r.Name,
			SSHURL:        *r.SSHURL,
			Private:       *r.Private,
			DefaultBranch: *r.DefaultBranch,
			RawJSON:       rawJSONBytes,
		})
	}

	return repos, nil
}

type GitHubRepository struct {
	Name          string
	Owner         string
	SSHURL        string
	Private       bool
	DefaultBranch string
	RawJSON       []byte
}

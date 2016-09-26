package revok

import (
	"encoding/json"

	"code.cloudfoundry.org/lager"
	"github.com/google/go-github/github"
)

//go:generate counterfeiter . GitHubClient

type GitHubClient interface {
	ListRepositories(lager.Logger) ([]GitHubRepository, error)
	RemainingRequests(lager.Logger) (int, error)
}

type client struct {
	ghClient *github.Client
}

func NewGitHubClient(
	ghClient *github.Client,
) GitHubClient {
	return &client{
		ghClient: ghClient,
	}
}

func (c *client) RemainingRequests(logger lager.Logger) (int, error) {
	rate, _, err := c.ghClient.RateLimits()
	if err != nil {
		logger.Error("failed-getting-stats", err)
		return 0, err
	}
	return rate.Core.Remaining, nil
}

func (c *client) ListRepositories(logger lager.Logger) ([]GitHubRepository, error) {
	logger = logger.Session("list-originalRepositories")

	opts := &github.RepositoryListOptions{
		ListOptions: github.ListOptions{PerPage: 30},
	}

	var repos []GitHubRepository

	for {
		rs, resp, err := c.ghClient.Repositories.List("", opts)
		if err != nil {
			logger.Error("failed", err, lager.Data{
				"fetching-page": opts.ListOptions.Page,
			})
			return nil, err
		}

		for _, repo := range rs {
			rawJSONBytes, err := json.Marshal(repo)
			if err != nil {
				logger.Error("failed-to-marshal-json", err)
				return nil, err
			}

			repos = append(repos, GitHubRepository{
				Name:          *repo.Name,
				Owner:         *repo.Owner.Login,
				SSHURL:        *repo.SSHURL,
				Private:       *repo.Private,
				DefaultBranch: *repo.DefaultBranch,
				RawJSON:       rawJSONBytes,
			})
		}

		if resp.NextPage == 0 {
			break
		}

		opts.ListOptions.Page = resp.NextPage
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

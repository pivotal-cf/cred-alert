package revok

import (
	"context"
	"encoding/json"

	"code.cloudfoundry.org/lager"
	"github.com/google/go-github/github"
)

type GitHubRepository struct {
	Name          string
	Owner         string
	SSHURL        string
	Private       bool
	DefaultBranch string
	RawJSON       []byte
}

type GitHubOrganization struct {
	Name string `json:"login"`
}

//go:generate counterfeiter . GithubService

type GithubService interface {
	ListRepositoriesByOrg(logger lager.Logger, orgName string) ([]GitHubRepository, error)
	ListRepositoriesByUser(logger lager.Logger, userName string) ([]GitHubRepository, error)
	GetRepo(logger lager.Logger, owner, repoName string) (*GitHubRepository, error)
}

type GitHubClient struct {
	ghClient *github.Client
}

func NewGitHubClient(
	ghClient *github.Client,
) *GitHubClient {
	return &GitHubClient{
		ghClient: ghClient,
	}
}

func (c *GitHubClient) ListRepositoriesByOrg(logger lager.Logger, orgName string) ([]GitHubRepository, error) {
	logger = logger.Session("list-repositories-by-org")

	opts := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 30},
	}

	var repos []GitHubRepository

	for {
		rs, resp, err := c.ghClient.Repositories.ListByOrg(context.TODO(), orgName, opts)
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

func (c *GitHubClient) ListRepositoriesByUser(logger lager.Logger, userName string) ([]GitHubRepository, error) {
	logger = logger.Session("list-repositories-by-user")

	opts := &github.RepositoryListOptions{
		ListOptions: github.ListOptions{PerPage: 30},
	}

	var repos []GitHubRepository

	for {
		rs, resp, err := c.ghClient.Repositories.List(context.TODO(), userName, opts)
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

func (c *GitHubClient) GetRepo(logger lager.Logger, owner, repoName string) (*GitHubRepository, error) {
	logger = logger.Session("get-repository-by-owner")
	var repo *GitHubRepository
	for {
		rs, resp, err := c.ghClient.Repositories.Get(context.TODO(), owner, repoName)

		if err != nil {
			logger.Error("failed", err, lager.Data{
				"fetching-repo":   repoName,
				"owner":           owner,
				"response-status": resp.Status,
			})
			return nil, err
		}

		rawJSONBytes, err := json.Marshal(repo)
		if err != nil {
			logger.Error("failed-to-marshal-json", err)
			return nil, err
		}

		repo = &GitHubRepository{
			Name:          *rs.Name,
			Owner:         *rs.Owner.Login,
			SSHURL:        *rs.SSHURL,
			Private:       *rs.Private,
			DefaultBranch: *rs.DefaultBranch,
			RawJSON:       rawJSONBytes,
		}
	}
	return repo, nil
}

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
	ghRepositoryService GithubRepositoryService
}

//go:generate counterfeiter . GithubRepositoryService

type GithubRepositoryService interface {
	ListByOrg(ctx context.Context, org string, opt *github.RepositoryListByOrgOptions) ([]*github.Repository, *github.Response, error)
	List(ctx context.Context, user string, opt *github.RepositoryListOptions) ([]*github.Repository, *github.Response, error)
	Get(ctx context.Context, owner, repo string) (*github.Repository, *github.Response, error)
}

func NewGitHubClient(
	ghRepositoryService GithubRepositoryService,
) *GitHubClient {
	return &GitHubClient{
		ghRepositoryService: ghRepositoryService,
	}
}

func (c *GitHubClient) ListRepositoriesByOrg(logger lager.Logger, orgName string) ([]GitHubRepository, error) {
	logger = logger.Session("list-repositories-by-org")

	opts := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 30},
	}

	var repos []GitHubRepository

	for {
		rs, resp, err := c.ghRepositoryService.ListByOrg(context.TODO(), orgName, opts)
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
		rs, resp, err := c.ghRepositoryService.List(context.TODO(), userName, opts)
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
	rs, resp, err := c.ghRepositoryService.Get(context.TODO(), owner, repoName)

	if err != nil {
		logger.Error("failed", err, lager.Data{
			"fetching-repo":   repoName,
			"owner":           owner,
			"response-status": resp.Status,
		})
		return nil, err
	}

	rawJSONBytes, err := json.Marshal(rs)
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
	return repo, nil
}

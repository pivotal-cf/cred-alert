package teamstr

import (
	"log"
	"sort"

	"github.com/deckarep/golang-set"
	"github.com/google/go-github/github"
)

//go:generate counterfeiter . OrgRepo

type OrgRepo interface {
	AddTeamRepo(team int, owner string, repo string, opt *github.OrganizationAddTeamRepoOptions) (*github.Response, error)
	ListTeamRepos(team int, opt *github.ListOptions) ([]*github.Repository, *github.Response, error)
}

//go:generate counterfeiter . RepoRepo

type RepoRepo interface {
	ListByOrg(org string, opt *github.RepositoryListByOrgOptions) ([]*github.Repository, *github.Response, error)
}

type Syncer struct {
	orgRepo  OrgRepo
	repoRepo RepoRepo
}

func NewSyncer(orgRepo OrgRepo, repoRepo RepoRepo) *Syncer {
	return &Syncer{
		orgRepo:  orgRepo,
		repoRepo: repoRepo,
	}
}

func (s *Syncer) Swim(orgName string, teamID int) error {
	orgRepos, err := s.orgRepos(orgName)
	if err != nil {
		return err
	}

	teamRepos, err := s.teamRepos(teamID)
	if err != nil {
		return err
	}

	reposToAdd := orgRepos.Difference(teamRepos).ToSlice()

	return s.sync(orgName, teamID, reposToAdd)
}

func (s *Syncer) orgRepos(orgName string) (mapset.Set, error) {
	opts := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 30},
	}

	var repos []*github.Repository

	for {
		rs, resp, err := s.repoRepo.ListByOrg(orgName, opts)
		if err != nil {
			return nil, err
		}

		repos = append(repos, rs...)

		if resp.NextPage == 0 {
			break
		}

		opts.ListOptions.Page = resp.NextPage
	}

	repoSet := mapset.NewSet()
	for _, repo := range repos {
		repoSet.Add(*repo.Name)
	}

	return repoSet, nil
}

func (s *Syncer) teamRepos(teamID int) (mapset.Set, error) {
	opts := &github.ListOptions{PerPage: 30}

	var repos []*github.Repository

	for {
		rs, resp, err := s.orgRepo.ListTeamRepos(teamID, opts)
		if err != nil {
			return nil, err
		}

		repos = append(repos, rs...)

		if resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}

	repoSet := mapset.NewSet()
	for _, repo := range repos {
		repoSet.Add(*repo.Name)
	}

	return repoSet, nil
}

func (s *Syncer) sync(orgName string, teamID int, reposToAdd []interface{}) error {
	strRepos := []string{}

	for _, repo := range reposToAdd {
		repoName := repo.(string)

		strRepos = append(strRepos, repoName)
	}

	sort.Strings(strRepos)

	for _, repo := range strRepos {
		log.Println("Adding repo: ", repo)
		_, err := s.orgRepo.AddTeamRepo(teamID, orgName, repo, &github.OrganizationAddTeamRepoOptions{
			Permission: "pull",
		})

		if err != nil {
			return err
		}
	}

	return nil
}

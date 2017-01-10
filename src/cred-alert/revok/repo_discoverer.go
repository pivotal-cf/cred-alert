package revok

import (
	"context"
	"cred-alert/db"
	"fmt"
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/ifrit"
)

type RepoDiscoverer struct {
	logger               lager.Logger
	workdir              string
	cloneMsgCh           chan CloneMsg
	ghClient             GitHubClient
	clock                clock.Clock
	interval             time.Duration
	repositoryRepository db.RepositoryRepository
}

func NewRepoDiscoverer(
	logger lager.Logger,
	workdir string,
	cloneMsgCh chan CloneMsg,
	ghClient GitHubClient,
	clock clock.Clock,
	interval time.Duration,
	repositoryRepository db.RepositoryRepository,
) ifrit.Runner {
	return &RepoDiscoverer{
		logger:               logger,
		workdir:              workdir,
		cloneMsgCh:           cloneMsgCh,
		ghClient:             ghClient,
		clock:                clock,
		interval:             interval,
		repositoryRepository: repositoryRepository,
	}
}

func (r *RepoDiscoverer) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := r.logger.Session("repo-discoverer")
	logger.Info("started")

	close(ready)

	timer := r.clock.NewTicker(r.interval)

	ctx, cancel := context.WithCancel(context.Background())

	defer func() {
		logger.Info("done")
		timer.Stop()
	}()

	r.work(logger, signals, cancel)

	for {
		select {
		case <-signals:
			cancel()
		case <-ctx.Done():
			return nil
		case <-timer.C():
			r.work(logger, signals, cancel)
		}
	}
}

type CloneMsg struct {
	Repository string
	Owner      string
	URL        string
}

func (r *RepoDiscoverer) work(logger lager.Logger, signals <-chan os.Signal, cancel context.CancelFunc) {
	logger = logger.Session("work")
	defer logger.Info("done")

	orgs, err := r.ghClient.ListOrganizations(logger)
	if err != nil {
		logger.Error("failed-to-get-github-orgs", err)
		return
	}

	var repos []GitHubRepository
	for _, o := range orgs {
		rs, err := r.ghClient.ListRepositoriesByOrg(logger, o.Name)
		if err != nil {
			logger.Error("failed-to-get-github-repositories-by-org", err)
			return
		}

		repos = append(repos, rs...)
	}

	dbRepos, err := r.repositoryRepository.All()
	if err != nil {
		logger.Error("failed-to-get-db-repositories", err)
		return
	}

	knownRepos := NewRepoSet(len(dbRepos))
	knownRepos.AddAll(dbRepos)

	for _, repo := range repos {
		select {
		case <-signals:
			cancel()
			return
		default:
			if knownRepos.Contains(repo) {
				continue
			}

			err = r.repositoryRepository.Create(&db.Repository{
				Owner:         repo.Owner,
				Name:          repo.Name,
				SSHURL:        repo.SSHURL,
				Private:       repo.Private,
				DefaultBranch: repo.DefaultBranch,
				RawJSON:       repo.RawJSON,
			})
			if err != nil {
				logger.Error("failed-to-create-repository", err, lager.Data{
					"owner":      repo.Owner,
					"repository": repo.Name,
				})
				continue
			}

			r.cloneMsgCh <- CloneMsg{
				Repository: repo.Name,
				Owner:      repo.Owner,
				URL:        repo.SSHURL,
			}
		}
	}
}

type repoSet struct {
	set map[string]struct{}
}

func NewRepoSet(sizeHint int) *repoSet {
	return &repoSet{
		set: make(map[string]struct{}, sizeHint),
	}
}

func (s *repoSet) AddAll(repos []db.Repository) {
	for _, repo := range repos {
		s.Add(repo)
	}
}

func (s *repoSet) Add(repo db.Repository) {
	s.set[s.key(repo.Owner, repo.Name)] = struct{}{}
}

func (s *repoSet) Contains(repo GitHubRepository) bool {
	_, found := s.set[s.key(repo.Owner, repo.Name)]

	return found
}

func (s *repoSet) key(owner string, name string) string {
	return fmt.Sprintf("%s/%s", owner, name)
}

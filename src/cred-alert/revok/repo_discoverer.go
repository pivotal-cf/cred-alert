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

	return nil
}

type CloneMsg struct {
	Repository string
	Owner      string
	URL        string
}

func (r *RepoDiscoverer) work(logger lager.Logger, signals <-chan os.Signal, cancel context.CancelFunc) {
	logger = logger.Session("work")
	defer logger.Info("done")

	repos, err := r.ghClient.ListRepositories(logger)
	if err != nil {
		logger.Error("failed", err)
		return
	}

	dbRepos, err := r.repositoryRepository.All()
	if err != nil {
		logger.Error("failed", err)
		return
	}

	knownRepos := make(map[string]struct{}, len(dbRepos))
	for _, existingRepo := range dbRepos {
		key := fmt.Sprintf("%s-%s", existingRepo.Owner, existingRepo.Name)
		knownRepos[key] = struct{}{}
	}

	for _, repo := range repos {
		select {
		case <-signals:
			cancel()
			return
		default:
			key := fmt.Sprintf("%s-%s", repo.Owner, repo.Name)
			if _, found := knownRepos[key]; found {
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

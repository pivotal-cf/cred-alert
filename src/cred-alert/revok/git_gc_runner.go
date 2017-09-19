package revok

import (
	"cred-alert/db"
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . GitGCClient

type GitGCClient interface {
	GC(repoPath string) error
}

type GitGCRunner struct {
	repoRepo      db.RepositoryRepository
	clock         clock.Clock
	gitClient     GitGCClient
	retryInterval time.Duration
	logger        lager.Logger
}

func NewGitGCRunner(
	logger lager.Logger,
	clock clock.Clock,
	repoRepo db.RepositoryRepository,
	gitClient GitGCClient,
	retryInterval time.Duration,
) *GitGCRunner {
	return &GitGCRunner{
		clock:         clock,
		gitClient:     gitClient,
		logger:        logger,
		repoRepo:      repoRepo,
		retryInterval: retryInterval,
	}
}

func (g *GitGCRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := g.logger.Session("git-gc-runner")

	close(ready)

	g.work(logger)

	ticker := g.clock.NewTicker(g.retryInterval)

	for {
		select {
		case <-ticker.C():
			g.work(logger)
		case <-signals:
			logger.Info("signalled")
			return nil
		}
	}
}

func (g *GitGCRunner) work(logger lager.Logger) {
	logger.Info("starting")
	defer logger.Info("complete")

	repos, err := g.repoRepo.All()
	if err != nil {
		logger.Error("failed-fetching-repos", err)
		return
	}

	for _, repo := range repos {
		if !repo.Cloned {
			continue
		}

		err := g.gitClient.GC(repo.Path)
		if err != nil {
			logger.Error("failed-running-git-gc", err, lager.Data{"repo": repo})
		}
	}
}

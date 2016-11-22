package revok

import (
	"context"
	"cred-alert/db"
	"cred-alert/gitclient"
	"cred-alert/kolsch"
	"cred-alert/sniff"
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	git "github.com/libgit2/git2go"
	"github.com/tedsuo/ifrit"
)

type headCredentialCounter struct {
	logger               lager.Logger
	repositoryRepository db.RepositoryRepository
	clock                clock.Clock
	interval             time.Duration
	gitClient            gitclient.Client
	sniffer              sniff.Sniffer
}

func NewHeadCredentialCounter(
	logger lager.Logger,
	repositoryRepository db.RepositoryRepository,
	clock clock.Clock,
	interval time.Duration,
	gitClient gitclient.Client,
	sniffer sniff.Sniffer,
) ifrit.Runner {
	return &headCredentialCounter{
		logger:               logger,
		repositoryRepository: repositoryRepository,
		clock:                clock,
		interval:             interval,
		gitClient:            gitClient,
		sniffer:              sniffer,
	}
}

func (c *headCredentialCounter) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := c.logger.Session("head-credential-counter")
	logger.Info("starting")

	close(ready)

	timer := c.clock.NewTicker(c.interval)

	ctx, cancel := context.WithCancel(context.Background())

	quietLogger := kolsch.NewLogger()

	c.work(cancel, signals, logger, quietLogger)

	for {
		select {
		case <-timer.C():
			c.work(cancel, signals, logger, quietLogger)
		case <-signals:
			cancel()
			return nil
		case <-ctx.Done():
			return nil
		}
	}
}

func (c *headCredentialCounter) work(
	cancel context.CancelFunc,
	signals <-chan os.Signal,
	logger lager.Logger,
	quietLogger lager.Logger,
) {
	repositories, err := c.repositoryRepository.All()
	if err != nil {
		logger.Error("failed-getting-all-repositories", err)
	}

	for i := range repositories {
		select {
		case <-signals:
			cancel()
			return
		default:
			repository := repositories[i]
			repoLogger := logger.WithData(lager.Data{
				"ref":  repository.DefaultBranch,
				"path": repository.Path,
			})

			credentialCounts, err := c.gitClient.BranchCredentialCounts(quietLogger, repository.Path, c.sniffer, git.BranchRemote)
			if err != nil {
				repoLogger.Error("failed-to-get-credential-counts", err)
				continue
			}

			err = c.repositoryRepository.UpdateCredentialCount(&repository, credentialCounts)
			if err != nil {
				repoLogger.Error("failed-to-update-credential-count", err)
				continue
			}

			repoLogger.Info("updated-credential-count")
		}
	}
}

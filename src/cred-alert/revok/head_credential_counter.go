package revok

import (
	"context"
	"cred-alert/db"
	"cred-alert/gitclient"
	"cred-alert/scanners"
	"cred-alert/scanners/filescanner"
	"cred-alert/sniff"
	"fmt"
	"io"
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
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

	c.work(logger, cancel, signals)

	for {
		select {
		case <-timer.C():
			c.work(logger, cancel, signals)
		case <-signals:
			cancel()
			return nil
		case <-ctx.Done():
			return nil
		}
	}

	return nil
}

func (c *headCredentialCounter) work(
	logger lager.Logger,
	cancel context.CancelFunc,
	signals <-chan os.Signal,
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

			r, w := io.Pipe()

			errCh := make(chan error)
			go func() {
				errCh <- c.gitClient.AllBlobsForRef(repository.Path, fmt.Sprintf("refs/remotes/origin/%s", repository.DefaultBranch), w)
			}()

			var credCount uint
			_ = c.sniffer.Sniff(
				logger,
				filescanner.New(r, ""), // no filename necessary
				func(lager.Logger, scanners.Violation) error {
					credCount++
					return nil
				})

			if err := <-errCh; err != nil {
				repoLogger.Error("failed-to-get-blobs", err)
				continue
			}

			err := c.repositoryRepository.UpdateCredentialCount(&repository, credCount)
			if err != nil {
				repoLogger.Error("failed-to-update-credential-count", err)
			}
		}
	}
}

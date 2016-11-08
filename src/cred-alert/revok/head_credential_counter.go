package revok

import (
	"context"
	"cred-alert/db"
	"cred-alert/gitclient"
	"cred-alert/kolsch"
	"cred-alert/scanners"
	"cred-alert/scanners/filescanner"
	"cred-alert/sniff"
	"fmt"
	"io"
	"os"
	"sync"
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

	quietLogger := kolsch.NewLogger()

	c.work(ctx, cancel, signals, logger, quietLogger)

	for {
		select {
		case <-timer.C():
			c.work(ctx, cancel, signals, logger, quietLogger)
		case <-signals:
			cancel()
			return nil
		case <-ctx.Done():
			return nil
		}
	}
}

func (c *headCredentialCounter) work(
	ctx context.Context,
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
		repository := repositories[i]
		repoLogger := logger.WithData(lager.Data{
			"ref":  repository.DefaultBranch,
			"path": repository.Path,
		})

		r, w := io.Pipe()

		errCh := make(chan error)
		go func() {
			errCh <- c.gitClient.AllBlobsForRef(ctx, repository.Path, fmt.Sprintf("refs/remotes/origin/%s", repository.DefaultBranch), w)
		}()

		wg := sync.WaitGroup{}
		wg.Add(1)
		var credCount uint
		go func() {
			defer wg.Done()
			_ = c.sniffer.Sniff(
				quietLogger,
				filescanner.New(r, ""), // no filename necessary
				func(lager.Logger, scanners.Violation) error {
					credCount++
					return nil
				})
		}()

		select {
		case err := <-errCh:
			if err != nil {
				if err == gitclient.ErrInterrupted {
					return
				}
				repoLogger.Error("failed-to-get-blobs", err)
				continue
			}
		case <-signals:
			cancel()
			return
		}

		wg.Wait()

		err := c.repositoryRepository.UpdateCredentialCount(&repository, credCount)
		if err != nil {
			repoLogger.Error("failed-to-update-credential-count", err)
			continue
		}

		repoLogger.Info("updated-credential-count", lager.Data{
			"count": credCount,
		})
	}
}

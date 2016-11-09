package revok

import (
	"cred-alert/db"
	"cred-alert/gitclient"
	"cred-alert/metrics"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/lager"

	git "github.com/libgit2/git2go"
	"github.com/tedsuo/ifrit"
)

type Cloner struct {
	logger               lager.Logger
	workdir              string
	workCh               chan CloneMsg
	gitClient            gitclient.Client
	repositoryRepository db.RepositoryRepository
	cloneSuccessCounter  metrics.Counter
	cloneFailedCounter   metrics.Counter
	scanSuccessCounter   metrics.Counter
	scanFailedCounter    metrics.Counter
	scanner              Scanner
}

func NewCloner(
	logger lager.Logger,
	workdir string,
	workCh chan CloneMsg,
	gitClient gitclient.Client,
	repositoryRepository db.RepositoryRepository,
	scanner Scanner,
	emitter metrics.Emitter,
) ifrit.Runner {
	return &Cloner{
		logger:               logger,
		workdir:              workdir,
		workCh:               workCh,
		gitClient:            gitClient,
		repositoryRepository: repositoryRepository,
		scanner:              scanner,
		scanSuccessCounter:   emitter.Counter("revok.cloner.scan.success"),
		scanFailedCounter:    emitter.Counter("revok.cloner.scan.failed"),
		cloneSuccessCounter:  emitter.Counter("revok.cloner.clone.success"),
		cloneFailedCounter:   emitter.Counter("revok.cloner.clone.failed"),
	}
}

func (c *Cloner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := c.logger.Session("cloner")
	logger.Info("started")

	close(ready)

	defer logger.Info("done")

	for {
		select {
		case msg := <-c.workCh:
			c.work(logger, msg)
		case <-signals:
			return nil
		}
	}
}

func (c *Cloner) work(logger lager.Logger, msg CloneMsg) {
	dest := filepath.Join(c.workdir, msg.Owner, msg.Repository)

	workLogger := logger.Session("work", lager.Data{
		"owner":       msg.Owner,
		"repo":        msg.Repository,
		"url":         msg.URL,
		"destination": dest,
	})
	defer workLogger.Info("done")

	_, err := c.gitClient.Clone(msg.URL, dest)
	if err != nil {
		workLogger.Error("failed-to-clone", err)
		c.cloneFailedCounter.Inc(workLogger)
		err = os.RemoveAll(dest)
		if err != nil {
			workLogger.Error("failed-to-clean-up", err)
		}
		return
	}

	c.cloneSuccessCounter.Inc(workLogger)

	err = c.repositoryRepository.MarkAsCloned(msg.Owner, msg.Repository, dest)
	if err != nil {
		workLogger.Error("failed-to-mark-as-cloned", err)
		return
	}

	_, err = c.repositoryRepository.Find(msg.Owner, msg.Repository)
	if err != nil {
		workLogger.Error("failed-to-find-db-repo", err)
		return
	}

	scannedOids := map[git.Oid]struct{}{}

	branches, err := c.gitClient.AllBranches(dest)
	if err != nil {
		workLogger.Error("failed-to-get-branches", err)
		return
	}

	for branchName, target := range branches {
		err = c.scanner.Scan(
			workLogger,
			msg.Owner,
			msg.Repository,
			scannedOids,
			branchName,
			target,
			"",
		)

		if err != nil {
			c.scanFailedCounter.Inc(workLogger)
		} else {
			c.scanSuccessCounter.Inc(workLogger)
		}
	}
}

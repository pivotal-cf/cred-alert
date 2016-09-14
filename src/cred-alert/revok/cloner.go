package revok

import (
	"cred-alert/db"
	"cred-alert/gitclient"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/lager"

	"github.com/tedsuo/ifrit"
)

type Cloner struct {
	logger               lager.Logger
	workdir              string
	workCh               chan CloneMsg
	gitClient            gitclient.Client
	repositoryRepository db.RepositoryRepository
}

func NewCloner(
	logger lager.Logger,
	workdir string,
	workCh chan CloneMsg,
	gitClient gitclient.Client,
	repositoryRepository db.RepositoryRepository,
) ifrit.Runner {
	return &Cloner{
		logger:               logger,
		workdir:              workdir,
		workCh:               workCh,
		gitClient:            gitClient,
		repositoryRepository: repositoryRepository,
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

	return nil
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

	err := c.gitClient.Clone(msg.URL, dest)
	if err != nil {
		workLogger.Error("failed-to-clone", err)
		err = os.RemoveAll(dest)
		if err != nil {
			workLogger.Error("failed-to-clean-up", err)
		}
	}

	err = c.repositoryRepository.MarkAsCloned(msg.Owner, msg.Repository, dest)
	if err != nil {
		workLogger.Error("failed-to-mark-as-cloned", err)
	}
}

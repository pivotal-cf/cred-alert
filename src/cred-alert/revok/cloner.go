package revok

import (
	"cred-alert/db"
	"cred-alert/gitclient"
	"cred-alert/kolsch"
	"cred-alert/metrics"
	"cred-alert/scanners"
	"cred-alert/scanners/dirscanner"
	"cred-alert/sniff"
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
	sniffer              sniff.Sniffer
	repositoryRepository db.RepositoryRepository
	scanRepository       db.ScanRepository
	successCounter       metrics.Counter
	failedCounter        metrics.Counter
}

func NewCloner(
	logger lager.Logger,
	workdir string,
	workCh chan CloneMsg,
	gitClient gitclient.Client,
	sniffer sniff.Sniffer,
	repositoryRepository db.RepositoryRepository,
	scanRepository db.ScanRepository,
	emitter metrics.Emitter,
) ifrit.Runner {
	return &Cloner{
		logger:               logger,
		workdir:              workdir,
		workCh:               workCh,
		gitClient:            gitClient,
		sniffer:              sniffer,
		repositoryRepository: repositoryRepository,
		scanRepository:       scanRepository,
		successCounter:       emitter.Counter(successMetric),
		failedCounter:        emitter.Counter(failedMetric),
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
		return
	}

	err = c.repositoryRepository.MarkAsCloned(msg.Owner, msg.Repository, dest)
	if err != nil {
		workLogger.Error("failed-to-mark-as-cloned", err)
		return
	}

	dbRepository, err := c.repositoryRepository.Find(msg.Owner, msg.Repository)
	if err != nil {
		workLogger.Error("failed-to-find-db-repo", err)
		return
	}

	scan := c.scanRepository.Start(workLogger, "dir-scan", &dbRepository, nil)
	scanner := dirscanner.New(
		func(logger lager.Logger, violation scanners.Violation) error {
			line := violation.Line
			scan.RecordCredential(db.Credential{
				Owner:      msg.Owner,
				Repository: msg.Repository,
				Path:       line.Path,
				LineNumber: line.LineNumber,
			})
			return nil
		},
		c.sniffer,
	)
	_ = scanner.Scan(kolsch.NewLogger(), dest)
	finishScan(workLogger, scan, c.successCounter, c.failedCounter)
}

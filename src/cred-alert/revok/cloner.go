package revok

import (
	"cred-alert/db"
	"cred-alert/gitclient"
	"cred-alert/kolsch"
	"cred-alert/metrics"
	"cred-alert/scanners"
	"cred-alert/scanners/diffscanner"
	"cred-alert/sniff"
	"os"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/lager"

	git "github.com/libgit2/git2go"
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

	repo, err := c.gitClient.Clone(msg.URL, dest)
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

	head, err := repo.Head()
	if err != nil {
		workLogger.Error("failed-to-get-head-of-repo", err)
		return
	}

	scannedOids := map[git.Oid]struct{}{}
	err = c.scanAncestors(kolsch.NewLogger(), workLogger, repo, dest, dbRepository, head.Target(), scannedOids)
	if err != nil {
		workLogger.Error("failed-to-scan", err)
	}
}

func (c *Cloner) scanAncestors(
	quietLogger lager.Logger,
	workLogger lager.Logger,
	repo *git.Repository,
	repoPath string,
	dbRepository db.Repository,
	child *git.Oid,
	scannedOids map[git.Oid]struct{},
) error {
	parents, err := c.gitClient.GetParents(repo, child)
	if err != nil {
		return err
	}

	if len(parents) == 0 {
		return c.scan(quietLogger, workLogger, repoPath, dbRepository, child, scannedOids)
	}

	for _, parent := range parents {
		if _, found := scannedOids[*parent]; found {
			continue
		}

		err = c.scan(quietLogger, workLogger, repoPath, dbRepository, child, scannedOids, parent)
		if err != nil {
			return err
		}

		return c.scanAncestors(quietLogger, workLogger, repo, repoPath, dbRepository, parent, scannedOids)
	}

	return nil
}

func (c *Cloner) scan(
	quietLogger lager.Logger,
	workLogger lager.Logger,
	repoPath string,
	dbRepository db.Repository,
	child *git.Oid,
	scannedOids map[git.Oid]struct{},
	parents ...*git.Oid,
) error {
	var parent *git.Oid
	if len(parents) == 1 {
		parent = parents[0]
	}

	diff, err := c.gitClient.Diff(repoPath, parent, child)
	if err != nil {
		return err
	}

	scan := c.scanRepository.Start(quietLogger, "diff-scan", &dbRepository, nil)
	c.sniffer.Sniff(
		quietLogger,
		diffscanner.NewDiffScanner(strings.NewReader(diff)),
		func(logger lager.Logger, violation scanners.Violation) error {
			line := violation.Line
			scan.RecordCredential(db.NewCredential(
				dbRepository.Owner,
				dbRepository.Name,
				"",
				line.Path,
				line.LineNumber,
				violation.Start,
				violation.End,
			))
			return nil
		},
	)

	scannedOids[*child] = struct{}{}

	finishScan(workLogger, scan, c.successCounter, c.failedCounter)

	return nil
}

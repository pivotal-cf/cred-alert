package revok

import (
	"cred-alert/db"
	"cred-alert/gitclient"
	"cred-alert/kolsch"
	"cred-alert/metrics"
	"cred-alert/scanners"
	"cred-alert/scanners/diffscanner"
	"cred-alert/sniff"
	"strings"

	"code.cloudfoundry.org/lager"
	git "github.com/libgit2/git2go"
)

//go:generate counterfeiter . Scanner

type Scanner interface {
	Scan(lager.Logger, string, string, string, string) error
}

type scanner struct {
	gitClient            gitclient.Client
	repositoryRepository db.RepositoryRepository
	scanRepository       db.ScanRepository
	sniffer              sniff.Sniffer
}

func NewScanner(
	gitClient gitclient.Client,
	repositoryRepository db.RepositoryRepository,
	scanRepository db.ScanRepository,
	sniffer sniff.Sniffer,
	emitter metrics.Emitter,
) Scanner {
	return &scanner{
		gitClient:            gitClient,
		repositoryRepository: repositoryRepository,
		scanRepository:       scanRepository,
		sniffer:              sniffer,
	}
}

func (s *scanner) Scan(
	logger lager.Logger,
	owner string,
	repository string,
	startSHA string,
	stopSHA string,
) error {
	dbRepository, err := s.repositoryRepository.Find(owner, repository)
	if err != nil {
		logger.Error("failed-to-find-db-repo", err)
		return err
	}

	repo, err := git.OpenRepository(dbRepository.Path)
	if err != nil {
		logger.Error("failed-to-open-repo", err)
		return err
	}

	startOid, err := git.NewOid(startSHA)
	if err != nil {
		logger.Error("failed-to-create-start-oid", err)
		return err
	}

	var stopOid *git.Oid
	if stopSHA != "" {
		var err error
		stopOid, err = git.NewOid(stopSHA)
		if err != nil {
			logger.Error("failed-to-create-stop-oid", err)
			return err
		}
	}

	quietLogger := kolsch.NewLogger()
	scan := s.scanRepository.Start(quietLogger, "repo-scan", &dbRepository, nil)

	scannedOids := map[git.Oid]struct{}{}
	err = s.scanAncestors(
		quietLogger,
		logger,
		repo,
		dbRepository,
		scan,
		scannedOids,
		startOid,
		stopOid,
	)
	if err != nil {
		logger.Error("failed-to-scan", err)
	}

	err = scan.Finish()
	if err != nil {
		logger.Error("failed-to-finish-scan", err)
		return err
	}

	return nil
}

func (s *scanner) scanAncestors(
	quietLogger lager.Logger,
	logger lager.Logger,
	repo *git.Repository,
	dbRepository db.Repository,
	scan db.ActiveScan,
	scannedOids map[git.Oid]struct{},
	child *git.Oid,
	stopPoint *git.Oid,
) error {
	parents, err := s.gitClient.GetParents(repo, child)
	if err != nil {
		return err
	}

	if len(parents) == 0 {
		return s.scan(quietLogger, logger, dbRepository, scan, scannedOids, child)
	}

	for _, parent := range parents {
		if _, found := scannedOids[*parent]; found {
			continue
		}

		err = s.scan(quietLogger, logger, dbRepository, scan, scannedOids, child, parent)
		if err != nil {
			return err
		}

		if stopPoint != nil && parent.Equal(stopPoint) {
			continue
		}

		return s.scanAncestors(quietLogger, logger, repo, dbRepository, scan, scannedOids, parent, stopPoint)
	}

	return nil
}

func (s *scanner) scan(
	quietLogger lager.Logger,
	logger lager.Logger,
	dbRepository db.Repository,
	scan db.ActiveScan,
	scannedOids map[git.Oid]struct{},
	child *git.Oid,
	parents ...*git.Oid,
) error {
	var parent *git.Oid
	if len(parents) == 1 {
		parent = parents[0]
	}

	diff, err := s.gitClient.Diff(dbRepository.Path, parent, child)
	if err != nil {
		return err
	}

	s.sniffer.Sniff(
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

	return nil
}

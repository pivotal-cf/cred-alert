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
	Scan(lager.Logger, string, string, string) error
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

	oid, err := git.NewOid(startSHA)
	if err != nil {
		logger.Error("failed-to-create-oid", err)
		return err
	}

	scannedOids := map[git.Oid]struct{}{}
	err = s.scanAncestors(kolsch.NewLogger(), logger, repo, dbRepository, oid, scannedOids)
	if err != nil {
		logger.Error("failed-to-scan", err)
	}

	return nil
}

func (s *scanner) scanAncestors(
	quietLogger lager.Logger,
	logger lager.Logger,
	repo *git.Repository,
	dbRepository db.Repository,
	child *git.Oid,
	scannedOids map[git.Oid]struct{},
) error {
	parents, err := s.gitClient.GetParents(repo, child)
	if err != nil {
		return err
	}

	if len(parents) == 0 {
		return s.scan(quietLogger, logger, dbRepository, child, scannedOids)
	}

	for _, parent := range parents {
		if _, found := scannedOids[*parent]; found {
			continue
		}

		err = s.scan(quietLogger, logger, dbRepository, child, scannedOids, parent)
		if err != nil {
			return err
		}

		return s.scanAncestors(quietLogger, logger, repo, dbRepository, parent, scannedOids)
	}

	return nil
}

func (s *scanner) scan(
	quietLogger lager.Logger,
	logger lager.Logger,
	dbRepository db.Repository,
	child *git.Oid,
	scannedOids map[git.Oid]struct{},
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

	scan := s.scanRepository.Start(quietLogger, "diff-scan", &dbRepository, nil)
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

	err = scan.Finish()
	if err != nil {
		logger.Error("failed-to-finish-scan", err)
		return err
	}

	return nil
}

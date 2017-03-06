package revok

import (
	"strings"

	"code.cloudfoundry.org/lager"

	"cred-alert/db"
	"cred-alert/gitclient"
	"cred-alert/kolsch"
	"cred-alert/scanners"
	"cred-alert/scanners/diffscanner"
	"cred-alert/sniff"
)

//go:generate counterfeiter . Scanner

type Scanner interface {
	Scan(lager.Logger, string, string, map[string]struct{}, string, string, string) ([]db.Credential, error)
}

type scanner struct {
	gitClient            gitclient.Client
	repositoryRepository db.RepositoryRepository
	scanRepository       db.ScanRepository
	credentialRepository db.CredentialRepository
	sniffer              sniff.Sniffer
}

func NewScanner(
	gitClient gitclient.Client,
	repositoryRepository db.RepositoryRepository,
	scanRepository db.ScanRepository,
	credentialRepository db.CredentialRepository,
	sniffer sniff.Sniffer,
) Scanner {
	return &scanner{
		gitClient:            gitClient,
		repositoryRepository: repositoryRepository,
		scanRepository:       scanRepository,
		credentialRepository: credentialRepository,
		sniffer:              sniffer,
	}
}

func (s *scanner) Scan(
	logger lager.Logger,
	owner string,
	repository string,
	scannedOids map[string]struct{},
	branch string,
	startSHA string,
	stopSHA string,
) ([]db.Credential, error) {
	dbRepository, err := s.repositoryRepository.MustFind(owner, repository)
	if err != nil {
		logger.Error("failed-to-find-db-repo", err)
		return nil, err
	}

	credentials, err := s.scan(logger, dbRepository, scannedOids, branch, startSHA, stopSHA)
	if err != nil {
		return nil, err
	}

	return credentials, nil
}

func (s *scanner) scan(
	logger lager.Logger,
	dbRepository db.Repository,
	scannedOids map[string]struct{},
	branch string,
	startSHA string,
	stopSHA string,
) ([]db.Credential, error) {
	quietLogger := kolsch.NewLogger()
	scan := s.scanRepository.Start(quietLogger, "repo-scan", branch, startSHA, stopSHA, &dbRepository, nil)

	var credentials []db.Credential

	scanFunc := func(child, parent string) error {
		diff, err := s.gitClient.Diff(dbRepository.Path, parent, child)
		if err != nil {
			return err
		}

		s.sniffer.Sniff(
			quietLogger,
			diffscanner.NewDiffScanner(strings.NewReader(diff)),
			func(logger lager.Logger, violation scanners.Violation) error {
				credential := db.Credential{
					Owner:      dbRepository.Owner,
					Repository: dbRepository.Name,
					SHA:        child,
					Path:       violation.Line.Path,
					LineNumber: violation.Line.LineNumber,
					MatchStart: violation.Start,
					MatchEnd:   violation.End,
					Private:    dbRepository.Private,
				}

				scan.RecordCredential(credential)
				credentials = append(credentials, credential)

				return nil
			},
		)

		scannedOids[child] = struct{}{}

		return nil
	}

	knownSHAs := map[string]struct{}{}
	shas, err := s.credentialRepository.UniqueSHAsForRepoAndRulesVersion(dbRepository, sniff.RulesVersion)
	for i := range shas {
		knownSHAs[shas[i]] = struct{}{}
	}

	err = s.scanAncestors(dbRepository.Path, scanFunc, scannedOids, knownSHAs, startSHA, stopSHA)
	if err != nil {
		logger.Error("failed-to-scan-ancestors", err, lager.Data{
			"start":      startSHA,
			"stop":       stopSHA,
			"repository": dbRepository.Name,
			"owner":      dbRepository.Owner,
		})
	}

	err = scan.Finish()
	if err != nil {
		logger.Error("failed-to-finish-scan", err)
		return nil, err
	}

	return credentials, nil
}

func (s *scanner) scanAncestors(
	repoPath string,
	scanFunc func(string, string) error,
	scannedOids map[string]struct{},
	knownSHAs map[string]struct{},
	child string,
	stopPoint string,
) error {
	if _, found := scannedOids[child]; found {
		return nil
	}

	if _, found := knownSHAs[child]; found {
		return nil
	}

	parents, err := s.gitClient.GetParents(repoPath, child)
	if err != nil {
		return err
	}

	if len(parents) == 0 {
		return scanFunc(child, "")
	}

	if len(parents) == 1 {
		err = scanFunc(child, parents[0])
		if err != nil {
			return err
		}
	}

	for _, parent := range parents {
		if stopPoint == parent {
			continue
		}

		err = s.scanAncestors(repoPath, scanFunc, scannedOids, knownSHAs, parent, stopPoint)
		if err != nil {
			return err
		}
	}

	return nil
}

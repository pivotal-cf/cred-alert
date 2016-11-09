package db

import (
	"cred-alert/sniff"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/jinzhu/gorm"
)

//go:generate counterfeiter . ScanRepository

type ScanRepository interface {
	Start(lager.Logger, string, string, string, string, *Repository, *Fetch) ActiveScan
	ScansNotYetRunWithVersion(lager.Logger, int) ([]PriorScan, error)
}

type scanRepository struct {
	db    *gorm.DB
	clock clock.Clock
}

func NewScanRepository(db *gorm.DB, clock clock.Clock) ScanRepository {
	return &scanRepository{
		db:    db,
		clock: clock,
	}
}

func (repo *scanRepository) Start(
	logger lager.Logger,
	scanType string,
	branch string,
	startSHA string,
	stopSHA string,
	repository *Repository,
	fetch *Fetch,
) ActiveScan {
	logger = logger.Session("start-scan", lager.Data{
		"type":          scanType,
		"rules-version": sniff.RulesVersion,
	})

	logger.Debug("starting")

	return &activeScan{
		logger: logger,
		clock:  repo.clock,
		tx:     repo.db.Begin(),

		repository: repository,
		fetch:      fetch,
		typee:      scanType,
		startTime:  repo.clock.Now(),

		branch:   branch,
		startSHA: startSHA,
		stopSHA:  stopSHA,
	}
}

//go:generate counterfeiter . ActiveScan

type ActiveScan interface {
	RecordCredential(Credential)
	Finish() error
}

type activeScan struct {
	logger lager.Logger
	clock  clock.Clock
	tx     *gorm.DB

	typee      string
	startTime  time.Time
	repository *Repository
	fetch      *Fetch

	branch   string
	startSHA string
	stopSHA  string

	credentials []Credential
}

func (s *activeScan) RecordCredential(credential Credential) {
	s.credentials = append(s.credentials, credential)
}

func (s *activeScan) Finish() error {
	scan := Scan{
		Type:         s.typee,
		RulesVersion: sniff.RulesVersion,
		ScanStart:    s.startTime,
		ScanEnd:      s.clock.Now(),
		Branch:       s.branch,
		StartSHA:     s.startSHA,
		StopSHA:      s.stopSHA,
		Credentials:  s.credentials,
	}

	// don't update the association on save, but actually save it on the scan
	if s.repository != nil && s.repository.ID != 0 {
		scan.RepositoryID = &s.repository.ID
	}

	if s.fetch != nil && s.fetch.ID != 0 {
		scan.FetchID = &s.fetch.ID
	}

	if err := s.tx.Save(&scan).Error; err != nil {
		s.tx.Rollback()
		return err
	}

	s.tx.Commit()

	return nil
}

type PriorScan struct {
	ID          int
	Branch      string
	StartSHA    string
	StopSHA     string
	Repository  string
	Owner       string
	Credentials []Credential
}

func (repo *scanRepository) ScansNotYetRunWithVersion(logger lager.Logger, version int) ([]PriorScan, error) {
	logger = logger.Session("prior-scans-for-rules-version", lager.Data{
		"version": version,
	})

	rows, err := repo.db.DB().Query(`
    SELECT s.id,
           s.branch,
           s.start_sha,
           s.stop_sha,
           r.owner,
           r.name
      FROM scans s
           JOIN repositories r
             ON s.repository_id = r.id
           JOIN (SELECT repository_id,
                        start_sha,
                        stop_sha,
                        MAX(rules_version) AS max_version
                   FROM scans
                  WHERE start_sha <> ''
                  GROUP BY 1, 2, 3
                 HAVING max_version = ? - 1) ood_scans
             ON s.repository_id = ood_scans.repository_id
                AND s.start_sha = ood_scans.start_sha
                AND s.stop_sha = ood_scans.stop_sha
                AND s.rules_version = ood_scans.max_version
	`, version)
	if err != nil {
		logger.Error("failed-to-execute-query", err)
		return nil, err
	}

	defer rows.Close()

	var (
		previousScans []PriorScan

		id             int
		branch         string
		startSHA       string
		stopSHA        string
		owner          string
		repositoryName string
	)

	for rows.Next() {
		scanErr := rows.Scan(&id, &branch, &startSHA, &stopSHA, &owner, &repositoryName)
		if scanErr != nil {
			logger.Error("failed-to-scan-row", err)
			return nil, scanErr
		}
		previousScans = append(previousScans, PriorScan{
			ID:         id,
			Branch:     branch,
			StartSHA:   startSHA,
			StopSHA:    stopSHA,
			Owner:      owner,
			Repository: repositoryName,
		})
	}

	return previousScans, nil
}

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
	Start(logger lager.Logger, scanType, startSHA, stopSHA string, repository *Repository, fetch *Fetch) ActiveScan
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

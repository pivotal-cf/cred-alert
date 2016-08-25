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
	Start(logger lager.Logger, scanType string) ActiveScan
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

func (repo *scanRepository) Start(logger lager.Logger, scanType string) ActiveScan {
	logger = logger.Session("start-scan", lager.Data{
		"type":          scanType,
		"rules-version": sniff.RulesVersion,
	})

	logger.Debug("starting")

	return &activeScan{
		logger: logger,
		clock:  repo.clock,
		tx:     repo.db.Begin(),

		typee:     scanType,
		startTime: repo.clock.Now(),
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

	typee     string
	startTime time.Time

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
		Credentials:  s.credentials,
	}

	if err := s.tx.Save(&scan).Error; err != nil {
		s.tx.Rollback()
		return err
	}

	s.tx.Commit()

	return nil
}

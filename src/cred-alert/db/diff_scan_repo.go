package db

import (
	"code.cloudfoundry.org/lager"
	"github.com/jinzhu/gorm"
)

//go:generate counterfeiter . DiffScanRepository

type DiffScanRepository interface {
	SaveDiffScan(lager.Logger, *DiffScan) error
}

type diffScanRepository struct {
	db *gorm.DB
}

func NewDiffScanRepository(db *gorm.DB) *diffScanRepository {
	return &diffScanRepository{db: db}
}

func (d *diffScanRepository) SaveDiffScan(logger lager.Logger, diffScan *DiffScan) error {
	logger = logger.Session("saving-diffscan", lager.Data{
		"owner":            diffScan.Owner,
		"repository":       diffScan.Repository,
		"from-commit":      diffScan.FromCommit,
		"to-commit":        diffScan.ToCommit,
		"credential-found": diffScan.CredentialFound,
	})
	logger.Debug("starting")

	err := d.db.Save(diffScan).Error
	if err != nil {
		logger.Error("failed", err)
	}

	logger.Debug("done")
	return err
}

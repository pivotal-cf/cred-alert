package db

import (
	"code.cloudfoundry.org/lager"
	"github.com/jinzhu/gorm"
)

//go:generate counterfeiter . CredentialRepository

type CredentialRepository interface {
	RegisterCredential(logger lager.Logger, credential *Credential) error
}

type credentialRepository struct {
	db *gorm.DB
}

func NewCredentialRepository(db *gorm.DB) CredentialRepository {
	return &credentialRepository{db: db}
}

func (repo *credentialRepository) RegisterCredential(logger lager.Logger, credential *Credential) error {
	logger = logger.Session("register-credential", lager.Data{
		"owner":           credential.Owner,
		"repository":      credential.Repository,
		"sha":             credential.SHA,
		"path":            credential.Path,
		"line-number":     credential.LineNumber,
		"scanning-method": credential.ScanningMethod,
		"rules-version":   credential.RulesVersion,
	})
	logger.Debug("starting")

	err := repo.db.Save(credential).Error
	if err != nil {
		logger.Error("failed", err)
	}

	logger.Debug("done")
	return err
}

package db_test

import (
	"cred-alert/db"
	"errors"

	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/jinzhu/gorm"
)

var _ = Describe("Credential Repository", func() {
	var (
		database *gorm.DB
		logger   *lagertest.TestLogger
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("commit-repository")
		dbRunner.Truncate()
		var err error
		database, err = dbRunner.GormDB()
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("CredentialRepository", func() {
		var (
			credentialRepository db.CredentialRepository
			credential           *db.Credential
		)

		BeforeEach(func() {
			credentialRepository = db.NewCredentialRepository(database)
			credential = &db.Credential{
				Owner:      "some-owner",
				Repository: "some-repository",
				SHA:        "abc123",
				Path:       "some-fake-path",
				LineNumber: 123,

				ScanningMethod: "some-scan",
				RulesVersion:   1,
			}
		})

		Describe("RegisterCredential", func() {
			It("saves a credential to the database", func() {
				err := credentialRepository.RegisterCredential(logger, credential)
				Expect(err).NotTo(HaveOccurred())

				savedCredential := db.Credential{}
				database.Last(&savedCredential)

				Expect(savedCredential.Owner).To(Equal(credential.Owner))
				Expect(savedCredential.Repository).To(Equal(credential.Repository))
				Expect(savedCredential.SHA).To(Equal(credential.SHA))
				Expect(savedCredential.Path).To(Equal(credential.Path))
				Expect(savedCredential.LineNumber).To(Equal(credential.LineNumber))
				Expect(savedCredential.ScanningMethod).To(Equal(credential.ScanningMethod))
				Expect(savedCredential.RulesVersion).To(Equal(credential.RulesVersion))
			})

			It("returns any error", func() {
				saveError := errors.New("saving credential error")
				database.AddError(saveError)
				err := credentialRepository.RegisterCredential(logger, credential)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(saveError))
			})

		})
	})
})

package db_test

import (
	"cred-alert/db"
	"cred-alert/sniff"
	"errors"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/jinzhu/gorm"
)

var _ = Describe("Scan Repository", func() {
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

	Describe("performing a scan", func() {
		var (
			scanRepository db.ScanRepository

			clock *fakeclock.FakeClock
		)

		BeforeEach(func() {
			clock = fakeclock.NewFakeClock(time.Now())
			scanRepository = db.NewScanRepository(database, clock)
		})

		It("works with no credentials", func() {
			startTime := clock.Now()

			scan := scanRepository.Start(logger, "scan-type")

			clock.Increment(time.Second * 5)

			err := scan.Finish()
			Expect(err).NotTo(HaveOccurred())

			savedScan := db.Scan{}
			database.Last(&savedScan)

			Expect(savedScan.Type).To(Equal("scan-type"))
			Expect(savedScan.RulesVersion).To(Equal(sniff.RulesVersion))
			Expect(savedScan.ScanStart).To(BeTemporally("~", startTime, time.Second))

			endTime := startTime.Add(5 * time.Second)
			Expect(savedScan.ScanEnd).To(BeTemporally("~", endTime, time.Second))
		})

		It("can record found credentials for a scan", func() {
			activeScan := scanRepository.Start(logger, "scan-type")

			credential := db.Credential{
				Owner:      "owner",
				Repository: "repo",
				SHA:        "sha",
				Path:       "this/is/a/path",
				LineNumber: 42,
			}

			activeScan.RecordCredential(credential)

			otherCredential := db.Credential{
				Owner:      "owner",
				Repository: "repo",
				SHA:        "sha",
				Path:       "this/is/an/other/path",
				LineNumber: 92,
			}

			activeScan.RecordCredential(otherCredential)

			err := activeScan.Finish()
			Expect(err).NotTo(HaveOccurred())

			savedScan := db.Scan{}
			database.Last(&savedScan)

			savedCredentials := []db.Credential{}
			err = database.Model(&savedScan).Related(&savedCredentials).Error
			Expect(err).NotTo(HaveOccurred())

			firstSavedCredential := savedCredentials[0]
			Expect(firstSavedCredential.Owner).To(Equal(credential.Owner))
			Expect(firstSavedCredential.Repository).To(Equal(credential.Repository))
			Expect(firstSavedCredential.SHA).To(Equal(credential.SHA))
			Expect(firstSavedCredential.Path).To(Equal(credential.Path))
			Expect(firstSavedCredential.LineNumber).To(Equal(credential.LineNumber))

			secondSavedCredential := savedCredentials[1]
			Expect(secondSavedCredential.Owner).To(Equal(otherCredential.Owner))
			Expect(secondSavedCredential.Repository).To(Equal(otherCredential.Repository))
			Expect(secondSavedCredential.SHA).To(Equal(otherCredential.SHA))
			Expect(secondSavedCredential.Path).To(Equal(otherCredential.Path))
			Expect(secondSavedCredential.LineNumber).To(Equal(otherCredential.LineNumber))
		})

		Context("when there is an error saving the scan", func() {
			var (
				saveError error
			)

			BeforeEach(func() {
				saveError = errors.New("disaster")
				database.AddError(saveError)
			})

			It("returns an error", func() {
				err := scanRepository.Start(logger, "scan-type").Finish()
				Expect(err).To(MatchError(saveError))
			})

			It("does not save any of the credentials from the scan", func() {
				scan := scanRepository.Start(logger, "scan-type")

				credential := db.Credential{
					Owner:      "owner",
					Repository: "repo",
					SHA:        "sha",
					Path:       "this/is/a/path",
					LineNumber: 42,
				}

				scan.RecordCredential(credential)

				scan.Finish()

				var count int
				savedCredentials := db.Credential{}
				database.Find(&savedCredentials).Count(&count)
				Expect(count).To(BeZero())

			})
		})
	})
})

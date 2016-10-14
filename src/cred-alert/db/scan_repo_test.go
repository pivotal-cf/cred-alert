package db_test

import (
	"cred-alert/db"
	"cred-alert/sniff"
	"encoding/json"
	"errors"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	uuid "github.com/satori/go.uuid"

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

			scan := scanRepository.Start(logger, "scan-type", "start-sha", "stop-sha", nil, nil)

			clock.Increment(time.Second * 5)

			err := scan.Finish()
			Expect(err).NotTo(HaveOccurred())

			savedScan := db.Scan{}
			database.Last(&savedScan)

			Expect(savedScan.Type).To(Equal("scan-type"))
			Expect(savedScan.RulesVersion).To(Equal(sniff.RulesVersion))
			Expect(savedScan.ScanStart).To(BeTemporally("~", startTime, time.Second))
			Expect(savedScan.StartSHA).To(Equal("start-sha"))
			Expect(savedScan.StopSHA).To(Equal("stop-sha"))

			endTime := startTime.Add(5 * time.Second)
			Expect(savedScan.ScanEnd).To(BeTemporally("~", endTime, time.Second))
		})

		It("can record found credentials for a scan", func() {
			activeScan := scanRepository.Start(logger, "scan-type", "start-sha", "stop-sha", nil, nil)

			credential := db.NewCredential("owner",
				"repo",
				"sha",
				"this/is/a/path",
				42,
				1,
				8,
				true,
			)

			activeScan.RecordCredential(credential)

			otherCredential := db.NewCredential(
				"owner",
				"repo",
				"sha",
				"this/is/an/other/path",
				92,
				31,
				38,
				false,
			)

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
			Expect(firstSavedCredential.MatchStart).To(Equal(credential.MatchStart))
			Expect(firstSavedCredential.MatchEnd).To(Equal(credential.MatchEnd))
			Expect(firstSavedCredential.Private).To(BeTrue())

			secondSavedCredential := savedCredentials[1]
			Expect(secondSavedCredential.Owner).To(Equal(otherCredential.Owner))
			Expect(secondSavedCredential.Repository).To(Equal(otherCredential.Repository))
			Expect(secondSavedCredential.SHA).To(Equal(otherCredential.SHA))
			Expect(secondSavedCredential.Path).To(Equal(otherCredential.Path))
			Expect(secondSavedCredential.LineNumber).To(Equal(otherCredential.LineNumber))
			Expect(secondSavedCredential.MatchStart).To(Equal(otherCredential.MatchStart))
			Expect(secondSavedCredential.MatchEnd).To(Equal(otherCredential.MatchEnd))
			Expect(secondSavedCredential.Private).To(BeFalse())
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
				err := scanRepository.Start(logger, "scan-type", "start-sha", "stop-sha", nil, nil).Finish()
				Expect(err).To(MatchError(saveError))
			})

			It("does not save any of the credentials from the scan", func() {
				scan := scanRepository.Start(logger, "scan-type", "start-sha", "stop-sha", nil, nil)

				credential := db.NewCredential(
					"owner",
					"repo",
					"sha",
					"this/is/a/path",
					42,
					22,
					33,
					false,
				)

				scan.RecordCredential(credential)

				scan.Finish()

				var count int
				savedCredentials := db.Credential{}
				database.Find(&savedCredentials).Count(&count)
				Expect(count).To(BeZero())
			})
		})

		Context("when the scan includes a repository and fetch", func() {
			var (
				repository *db.Repository
				fetch      *db.Fetch
				scan       db.ActiveScan
			)

			BeforeEach(func() {
				repoJSON := map[string]interface{}{
					"path": "path-to-repo-on-disk",
					"name": "repo-name",
					"owner": map[string]interface{}{
						"login": "owner-name",
					},
					"private":        true,
					"default_branch": "master",
				}

				repoJSONBytes, err := json.Marshal(repoJSON)
				Expect(err).NotTo(HaveOccurred())
				repository = &db.Repository{
					Name:          uuid.NewV4().String(),
					Owner:         "owner-name",
					Path:          "path-to-repo-on-disk",
					SSHURL:        "repo-ssh-url",
					Private:       true,
					DefaultBranch: "master",
					RawJSON:       repoJSONBytes,
				}

				err = database.FirstOrCreate(repository, *repository).Error
				Expect(err).NotTo(HaveOccurred())

				changes := map[string][]string{
					"change": {"from", "to"},
				}

				bs, err := json.Marshal(changes)
				Expect(err).NotTo(HaveOccurred())

				fetch = &db.Fetch{
					RepositoryID: repository.ID,
					Path:         "path-to-repo-on-disk",
					Changes:      bs,
				}

				err = database.Save(fetch).Error
				Expect(err).NotTo(HaveOccurred())

				scan = scanRepository.Start(logger, "scan-type", "start-sha", "stop-sha", repository, fetch)
			})

			It("saves both appropriately on Finish", func() {
				scan.Finish()

				savedScan := db.Scan{}
				database.Last(&savedScan)

				var count uint
				database.Model(db.Scan{}).Where("repository_id = ? AND fetch_id = ?", repository.ID, fetch.ID).Count(&count)
				Expect(count).To(Equal(uint(1)))
			})
		})
	})
})

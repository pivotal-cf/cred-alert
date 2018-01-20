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
	uuid "github.com/satori/go.uuid"

	"github.com/jinzhu/gorm"
)

var _ = Describe("Scan Repository", func() {
	var (
		database       *gorm.DB
		clock          *fakeclock.FakeClock
		logger         *lagertest.TestLogger
		scanRepository db.ScanRepository
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("commit-repository")

		dbRunner.Truncate()

		var err error
		database, err = dbRunner.GormDB()
		Expect(err).NotTo(HaveOccurred())

		clock = fakeclock.NewFakeClock(time.Now())
		scanRepository = db.NewScanRepository(database, clock)
	})

	Describe("performing a scan", func() {
		It("works with no credentials", func() {
			startTime := clock.Now()

			scan := scanRepository.Start(logger, "scan-type", "branch", "start-sha", "stop-sha", nil)

			clock.Increment(time.Second * 5)

			err := scan.Finish()
			Expect(err).NotTo(HaveOccurred())

			savedScan := db.Scan{}
			database.Last(&savedScan)

			Expect(savedScan.Type).To(Equal("scan-type"))
			Expect(savedScan.RulesVersion).To(Equal(sniff.RulesVersion))
			Expect(savedScan.ScanStart).To(BeTemporally("~", startTime, time.Second))
			Expect(savedScan.Branch).To(Equal("branch"))
			Expect(savedScan.StartSHA).To(Equal("start-sha"))
			Expect(savedScan.StopSHA).To(Equal("stop-sha"))

			endTime := startTime.Add(5 * time.Second)
			Expect(savedScan.ScanEnd).To(BeTemporally("~", endTime, time.Second))
		})

		It("can record found credentials for a scan", func() {
			activeScan := scanRepository.Start(logger, "scan-type", "branch", "start-sha", "stop-sha", nil)

			credential := db.Credential{
				Owner:      "owner",
				Repository: "repo",
				SHA:        "sha",
				Path:       "this/is/a/path",
				LineNumber: 42,
				MatchStart: 1,
				MatchEnd:   8,
				Private:    true,
			}

			activeScan.RecordCredential(credential)

			otherCredential := db.Credential{
				Owner:      "owner",
				Repository: "repo",
				SHA:        "sha",
				Path:       "this/is/an/other/path",
				LineNumber: 92,
				MatchStart: 31,
				MatchEnd:   38,
				Private:    false,
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
				err := scanRepository.Start(logger, "scan-type", "branch", "start-sha", "stop-sha", nil).Finish()
				Expect(err).To(MatchError(saveError))
			})

			It("does not save any of the credentials from the scan", func() {
				scan := scanRepository.Start(logger, "scan-type", "branch", "start-sha", "stop-sha", nil)

				credential := db.Credential{
					Owner:      "owner",
					Repository: "repo",
					SHA:        "sha",
					Path:       "this/is/a/path",
					LineNumber: 42,
					MatchStart: 22,
					MatchEnd:   33,
					Private:    false,
				}

				scan.RecordCredential(credential)

				scan.Finish()

				var count int
				savedCredentials := db.Credential{}
				database.Find(&savedCredentials).Count(&count)
				Expect(count).To(BeZero())
			})
		})

		Context("when the scan includes a repository", func() {
			var (
				repository *db.Repository
				scan       db.ActiveScan
			)

			BeforeEach(func() {
				repository = &db.Repository{
					Name:          uuid.Must(uuid.NewV4()).String(),
					Owner:         "owner-name",
					Path:          "path-to-repo-on-disk",
					SSHURL:        "repo-ssh-url",
					Private:       true,
					DefaultBranch: "master",
				}

				err := database.FirstOrCreate(repository, *repository).Error
				Expect(err).NotTo(HaveOccurred())

				scan = scanRepository.Start(logger, "scan-type", "branch", "start-sha", "stop-sha", repository)
			})

			It("saves the repository appropriately on Finish", func() {
				scan.Finish()

				savedScan := db.Scan{}
				database.Last(&savedScan)

				var count uint
				database.Model(db.Scan{}).Where("repository_id = ?", repository.ID).Count(&count)
				Expect(count).To(Equal(uint(1)))
			})
		})
	})

	Describe("ScansNotYetRunWithVersion", func() {
		var repository *db.Repository

		BeforeEach(func() {
			_, err := database.DB().Exec(`
					INSERT INTO repositories (name, owner)
					VALUES (
						'some-repo',
						'some-owner'
					)
				`)
			Expect(err).NotTo(HaveOccurred())

			var repositoryID uint
			err = database.DB().QueryRow(`SELECT id FROM repositories ORDER BY id DESC LIMIT 1`).Scan(&repositoryID)
			Expect(err).NotTo(HaveOccurred())

			repository = &db.Repository{
				Model: db.Model{
					ID: repositoryID,
				},
			}
		})

		It("returns nothing when there are no scans present for the rules version", func() {
			priorScans, err := scanRepository.ScansNotYetRunWithVersion(logger, 6)
			Expect(err).NotTo(HaveOccurred())
			Expect(priorScans).To(BeEmpty())
		})

		Context("when there are scans with a startSHA present for the rules version", func() {
			var expectedScanID int

			BeforeEach(func() {
				// two v5 scans
				err := scanRepository.Start(logger, "scan-type", "branch", "some-start-sha", "some-stop-sha", repository).Finish()
				Expect(err).NotTo(HaveOccurred())
				err = database.DB().QueryRow(`SELECT id FROM scans ORDER BY id DESC LIMIT 1`).Scan(&expectedScanID)
				Expect(err).NotTo(HaveOccurred())

				err = scanRepository.Start(logger, "scan-type", "branch", "some-other-start-sha", "some-other-stop-sha", repository).Finish()
				Expect(err).NotTo(HaveOccurred())

				database.DB().Exec(`UPDATE scans SET rules_version = 5`)

				// add another scan, same as the second above, but for v6
				err = scanRepository.Start(logger, "scan-type", "branch", "some-other-start-sha", "some-other-stop-sha", repository).Finish()
				Expect(err).NotTo(HaveOccurred())

				var v6ScanID int
				err = database.DB().QueryRow(`SELECT id FROM scans ORDER BY id DESC LIMIT 1`).Scan(&v6ScanID)
				Expect(err).NotTo(HaveOccurred())

				database.DB().Exec(`UPDATE scans SET rules_version = 6 WHERE id = ?`, v6ScanID)
			})

			It("returns scans for that rules version", func() {
				priorScans, err := scanRepository.ScansNotYetRunWithVersion(logger, 6)
				Expect(err).NotTo(HaveOccurred())

				// this scan has no v6 scan
				Expect(priorScans).To(Equal([]db.PriorScan{
					{
						ID:         expectedScanID,
						Branch:     "branch",
						StartSHA:   "some-start-sha",
						StopSHA:    "some-stop-sha",
						Owner:      "some-owner",
						Repository: "some-repo",
					},
				}))
			})
		})
	})
})

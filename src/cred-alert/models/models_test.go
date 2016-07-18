package models_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-golang/lager/lagertest"

	"cred-alert/models"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

var _ = Describe("Database Connections", func() {
	var (
		db           *gorm.DB
		dbFileHandle *os.File
		logger       *lagertest.TestLogger
	)

	BeforeEach(func() {
		var err error
		dbFileHandle, err = ioutil.TempFile("", "test.db")
		Expect(err).NotTo(HaveOccurred())

		err = dbFileHandle.Close()
		Expect(err).NotTo(HaveOccurred())

		db, err = gorm.Open("sqlite3", dbFileHandle.Name())
		Expect(err).NotTo(HaveOccurred())
		db.AutoMigrate(&models.DiffScan{}, &models.Commit{})
		logger = lagertest.NewTestLogger("commit-repository")
	})

	AfterEach(func() {
		db.Close()
		os.Remove(dbFileHandle.Name())
	})

	Describe("auto-migrations", func() {
		It("creates the DiffScan table", func() {
			Expect(db.HasTable(&models.DiffScan{})).To(BeTrue())
		})

		It("creates the Commit table", func() {
			Expect(db.HasTable(&models.Commit{})).To(BeTrue())
		})
	})

	Describe("CommitRepository", func() {
		var (
			commitRepository models.CommitRepository
			fakeCommit       *models.Commit
		)

		BeforeEach(func() {
			// commitRepository = models.NewCommitRepository(fakeDB)
			commitRepository = models.NewCommitRepository(db)
			fakeCommit = &models.Commit{
				SHA:       "abc123",
				Timestamp: time.Now(),
				Org:       "my-org",
				Repo:      "my-repo",
			}
		})

		Describe("RegisterCommit", func() {
			It("Saves a commit to the db", func() {
				commitRepository.RegisterCommit(logger, fakeCommit)
				// Expect(fakeDB.SaveCallCount()).To(Equal(1))
				// savedCommit := fakeDB.SaveArgsForCall(0)
				var savedCommit *models.Commit
				savedCommit = &models.Commit{}
				db.Last(&savedCommit)
				Expect(savedCommit.SHA).To(Equal(fakeCommit.SHA))
				Expect(savedCommit.Org).To(Equal(fakeCommit.Org))
				Expect(savedCommit.Repo).To(Equal(fakeCommit.Repo))
				Expect(savedCommit.Timestamp.Unix()).To(Equal(fakeCommit.Timestamp.Unix()))
			})

			It("returns any error", func() {
				saveError := errors.New("save error")
				db.AddError(saveError)
				err := commitRepository.RegisterCommit(logger, fakeCommit)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(saveError))
			})
		})

		Describe("IsCommitRegistered", func() {
			It("Returns true if a commit has been registered", func() {
				commitRepository.RegisterCommit(logger, fakeCommit)
				isRegistered, err := commitRepository.IsCommitRegistered(logger, "abc123")
				Expect(err).ToNot(HaveOccurred())
				Expect(isRegistered).To(BeTrue())
			})

			It("Returns false if a commit has not been registered", func() {
				commitRepository.RegisterCommit(logger, fakeCommit)
				isRegistered, err := commitRepository.IsCommitRegistered(logger, "wrong-sha")
				Expect(err).ToNot(HaveOccurred())
				Expect(isRegistered).To(BeFalse())
			})

			It("Returns any errors", func() {
				findError := errors.New("find commit error")
				db.AddError(findError)
				_, err := commitRepository.IsCommitRegistered(logger, "abc123")
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(findError))
			})
		})
	})

	Describe("DiffScanRepository", func() {
		var (
			diffScanRepository models.DiffScanRepository
			fakeDiffScan       *models.DiffScan
			taskID             string
		)

		BeforeEach(func() {
			diffScanRepository = models.NewDiffScanRepository(db)
			taskID = "some-guid"
			fakeDiffScan = &models.DiffScan{
				Org:             "my-org",
				Repo:            "my-repo",
				FromCommit:      "sha-1",
				ToCommit:        "sha-2",
				Timestamp:       time.Now(),
				TaskID:          taskID,
				CredentialFound: false,
			}
		})

		It("Saves a diff scan", func() {
			err := diffScanRepository.SaveDiffScan(logger, fakeDiffScan)
			Expect(err).ToNot(HaveOccurred())
			var diffs []models.DiffScan
			err = db.Where(&models.DiffScan{TaskID: taskID}).First(&diffs).Error
			Expect(err).ToNot(HaveOccurred())
			Expect(diffs).To(HaveLen(1))
			Expect(diffs[0].TaskID).To(Equal(taskID))
		})

		It("Returns any error", func() {
			findError := errors.New("save diff error")
			db.AddError(findError)
			err := diffScanRepository.SaveDiffScan(logger, fakeDiffScan)
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(findError))
		})

		It("should log successfully saving", func() {
			diffScanRepository.SaveDiffScan(logger, fakeDiffScan)
			Expect(logger).To(gbytes.Say("saving-diffscan"))
			Expect(logger).To(gbytes.Say("successfully-saved-diffscan"))
			Expect(logger).To(gbytes.Say(fmt.Sprintf(`"credential-found":%v`, fakeDiffScan.CredentialFound)))
			Expect(logger).To(gbytes.Say(fmt.Sprintf(`"from-commit":"%s"`, fakeDiffScan.FromCommit)))
			Expect(logger).To(gbytes.Say(fmt.Sprintf(`"org":"%s"`, fakeDiffScan.Org)))
			Expect(logger).To(gbytes.Say(fmt.Sprintf(`"repo":"%s"`, fakeDiffScan.Repo)))
			Expect(logger).To(gbytes.Say(fmt.Sprintf(`"scan-timestamp":%d`, fakeDiffScan.Timestamp.Unix())))
			Expect(logger).To(gbytes.Say(fmt.Sprintf(`"task-id":"%s"`, fakeDiffScan.TaskID)))
			Expect(logger).To(gbytes.Say(fmt.Sprintf(`"to-commit":"%s"`, fakeDiffScan.ToCommit)))
		})

		It("should log error saving", func() {
			findError := errors.New("save diff error")
			db.AddError(findError)
			diffScanRepository.SaveDiffScan(logger, fakeDiffScan)
			Expect(logger).To(gbytes.Say("error-saving-diffscan"))
		})
	})
})

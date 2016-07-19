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
		logger = lagertest.NewTestLogger("commit-repository")

		var err error
		dbFileHandle, err = ioutil.TempFile("", "test.db")
		Expect(err).NotTo(HaveOccurred())

		err = dbFileHandle.Close()
		Expect(err).NotTo(HaveOccurred())

		db, err = gorm.Open("sqlite3", dbFileHandle.Name())
		Expect(err).NotTo(HaveOccurred())
		db.AutoMigrate(&models.DiffScan{}, &models.Commit{})
		db.LogMode(false)
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
			repoName         string
			repoOwner        string
		)

		BeforeEach(func() {
			repoName = "my-repo"
			repoOwner = "my-org"

			commitRepository = models.NewCommitRepository(db)
			fakeCommit = &models.Commit{
				SHA:        "abc123",
				Timestamp:  time.Now(),
				Owner:      repoOwner,
				Repository: repoName,
			}
		})

		Describe("RegisterCommit", func() {
			It("Saves a commit to the db", func() {
				commitRepository.RegisterCommit(logger, fakeCommit)
				var savedCommit *models.Commit
				savedCommit = &models.Commit{}
				db.Last(&savedCommit)
				Expect(savedCommit.SHA).To(Equal(fakeCommit.SHA))
				Expect(savedCommit.Owner).To(Equal(fakeCommit.Owner))
				Expect(savedCommit.Repository).To(Equal(fakeCommit.Repository))
				Expect(savedCommit.Timestamp.Unix()).To(Equal(fakeCommit.Timestamp.Unix()))
			})

			It("returns any error", func() {
				saveError := errors.New("saving commit error")
				db.AddError(saveError)
				err := commitRepository.RegisterCommit(logger, fakeCommit)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(saveError))
			})

			It("should log when successfully registering", func() {
				commitRepository.RegisterCommit(logger, fakeCommit)
				Expect(logger).To(gbytes.Say("registering-commit.done"))
				Expect(logger).To(gbytes.Say(fmt.Sprintf(`"commit-timestamp":%d`, fakeCommit.Timestamp.Unix())))
				Expect(logger).To(gbytes.Say(fmt.Sprintf(`"owner":"%s"`, fakeCommit.Owner)))
				Expect(logger).To(gbytes.Say(fmt.Sprintf(`"repository":"%s"`, fakeCommit.Repository)))
				Expect(logger).To(gbytes.Say(fmt.Sprintf(`"sha":"%s"`, fakeCommit.SHA)))
			})

			It("should log error registering", func() {
				saveError := errors.New("saving commit error")
				db.AddError(saveError)
				commitRepository.RegisterCommit(logger, fakeCommit)
				Expect(logger).To(gbytes.Say("registering-commit.failed"))
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

			It("should log error registering", func() {
				saveError := errors.New("find commit error")
				db.AddError(saveError)
				commitRepository.IsCommitRegistered(logger, "abc123")
				Expect(logger).To(gbytes.Say("finding-commit.failed"))
				Expect(logger).To(gbytes.Say(fmt.Sprintf(`"sha":"%s"`, "abc123")))
			})
		})

		Describe("IsRepoRegistered", func() {
			It("Returns true if a repo has been registered", func() {
				commitRepository.RegisterCommit(logger, fakeCommit)
				isRegistered, err := commitRepository.IsRepoRegistered(logger, repoOwner, repoName)
				Expect(err).ToNot(HaveOccurred())
				Expect(isRegistered).To(BeTrue())
			})

			It("Returns false if a repo has not been registered", func() {
				commitRepository.RegisterCommit(logger, fakeCommit)
				isRegistered, err := commitRepository.IsRepoRegistered(logger, repoOwner, "wrong-repo")
				Expect(err).ToNot(HaveOccurred())
				Expect(isRegistered).To(BeFalse())

				isRegistered, err = commitRepository.IsRepoRegistered(logger, "wrong-org", repoName)
				Expect(err).ToNot(HaveOccurred())
				Expect(isRegistered).To(BeFalse())
			})

			It("Returns and logs any errors", func() {
				findError := errors.New("find repo error")
				db.AddError(findError)
				_, err := commitRepository.IsRepoRegistered(logger, repoOwner, repoName)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(findError))

				Expect(logger).To(gbytes.Say("finding-repo"))
				Expect(logger).To(gbytes.Say("error-finding-repo"))
				Expect(logger).To(gbytes.Say(fmt.Sprintf(`"repository":"%s"`, repoName)))
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

package db_test

import (
	"cred-alert/db"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"code.cloudfoundry.org/lager/lagertest"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

var _ = Describe("Database Connections", func() {
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

	Describe("auto-migrations", func() {
		It("creates the DiffScan table", func() {
			Expect(database.HasTable(&db.DiffScan{})).To(BeTrue())
		})

		It("creates the Commit table", func() {
			Expect(database.HasTable(&db.Commit{})).To(BeTrue())
		})
	})

	Describe("CommitRepository", func() {
		var (
			commitRepository db.CommitRepository
			fakeCommit       *db.Commit
			repoName         string
			repoOwner        string
		)

		BeforeEach(func() {
			repoName = "my-repo"
			repoOwner = "my-owner"

			commitRepository = db.NewCommitRepository(database)
			fakeCommit = &db.Commit{
				SHA:        "abc123",
				Owner:      repoOwner,
				Repository: repoName,
			}
		})

		Describe("RegisterCommit", func() {
			It("Saves a commit to the database", func() {
				commitRepository.RegisterCommit(logger, fakeCommit)
				var savedCommit *db.Commit
				savedCommit = &db.Commit{}
				database.Last(&savedCommit)
				Expect(savedCommit.SHA).To(Equal(fakeCommit.SHA))
				Expect(savedCommit.Owner).To(Equal(fakeCommit.Owner))
				Expect(savedCommit.Repository).To(Equal(fakeCommit.Repository))
			})

			It("returns any error", func() {
				saveError := errors.New("saving commit error")
				database.AddError(saveError)
				err := commitRepository.RegisterCommit(logger, fakeCommit)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(saveError))
			})

			It("should log when successfully registering", func() {
				commitRepository.RegisterCommit(logger, fakeCommit)
				Expect(logger).To(gbytes.Say("registering-commit.done"))
				Expect(logger).To(gbytes.Say(fmt.Sprintf(`"owner":"%s"`, fakeCommit.Owner)))
				Expect(logger).To(gbytes.Say(fmt.Sprintf(`"repository":"%s"`, fakeCommit.Repository)))
				Expect(logger).To(gbytes.Say(fmt.Sprintf(`"sha":"%s"`, fakeCommit.SHA)))
			})

			It("should log error registering", func() {
				saveError := errors.New("saving commit error")
				database.AddError(saveError)
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
				database.AddError(findError)
				_, err := commitRepository.IsCommitRegistered(logger, "abc123")
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(findError))
			})

			It("should log error registering", func() {
				saveError := errors.New("find commit error")
				database.AddError(saveError)
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

				isRegistered, err = commitRepository.IsRepoRegistered(logger, "wrong-owner", repoName)
				Expect(err).ToNot(HaveOccurred())
				Expect(isRegistered).To(BeFalse())
			})

			It("Returns and logs any errors", func() {
				findError := errors.New("find repo error")
				database.AddError(findError)
				_, err := commitRepository.IsRepoRegistered(logger, repoOwner, repoName)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(findError))

				Expect(logger).To(gbytes.Say("finding-repo"))
				Expect(logger).To(gbytes.Say("finding-repo.failed"))
				Expect(logger).To(gbytes.Say(fmt.Sprintf(`"repository":"%s"`, repoName)))
			})
		})
	})

	Describe("DiffScanRepository", func() {
		var (
			diffScanRepository db.DiffScanRepository
			fakeDiffScan       *db.DiffScan
		)

		BeforeEach(func() {
			diffScanRepository = db.NewDiffScanRepository(database)
			fakeDiffScan = &db.DiffScan{
				Owner:           "my-org",
				Repository:      "my-repo",
				FromCommit:      "sha-1",
				ToCommit:        "sha-2",
				CredentialFound: false,
			}
		})

		It("Saves a diff scan", func() {
			err := diffScanRepository.SaveDiffScan(logger, fakeDiffScan)
			Expect(err).ToNot(HaveOccurred())
			var diffs []db.DiffScan
			err = database.Last(&diffs).Error
			Expect(err).ToNot(HaveOccurred())
			Expect(diffs).To(HaveLen(1))
			Expect(diffs[0].Owner).To(Equal(fakeDiffScan.Owner))
			Expect(diffs[0].Repository).To(Equal(fakeDiffScan.Repository))
			Expect(diffs[0].ToCommit).To(Equal(fakeDiffScan.ToCommit))
			Expect(diffs[0].FromCommit).To(Equal(fakeDiffScan.FromCommit))
		})

		It("Returns any error", func() {
			findError := errors.New("save diff error")
			database.AddError(findError)
			err := diffScanRepository.SaveDiffScan(logger, fakeDiffScan)
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(findError))
		})

		It("should log successfully saving", func() {
			diffScanRepository.SaveDiffScan(logger, fakeDiffScan)
			Expect(logger).To(gbytes.Say("saving-diffscan"))
			Expect(logger).To(gbytes.Say("saving-diffscan.done"))
			Expect(logger).To(gbytes.Say(fmt.Sprintf(`"credential-found":%v`, fakeDiffScan.CredentialFound)))
			Expect(logger).To(gbytes.Say(fmt.Sprintf(`"from-commit":"%s"`, fakeDiffScan.FromCommit)))
			Expect(logger).To(gbytes.Say(fmt.Sprintf(`"owner":"%s"`, fakeDiffScan.Owner)))
			Expect(logger).To(gbytes.Say(fmt.Sprintf(`"repository":"%s"`, fakeDiffScan.Repository)))
			Expect(logger).To(gbytes.Say(fmt.Sprintf(`"to-commit":"%s"`, fakeDiffScan.ToCommit)))
		})

		It("should log error saving", func() {
			findError := errors.New("save diff error")
			database.AddError(findError)
			diffScanRepository.SaveDiffScan(logger, fakeDiffScan)
			Expect(logger).To(gbytes.Say("saving-diffscan.failed"))
		})
	})
})

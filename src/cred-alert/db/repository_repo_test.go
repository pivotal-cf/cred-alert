package db_test

import (
	"cred-alert/db"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	"github.com/jinzhu/gorm"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RepositoryRepo", func() {
	var (
		repo     db.RepositoryRepository
		database *gorm.DB
		logger   lager.Logger
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("reporepo")
		var err error
		database, err = dbRunner.GormDB()
		Expect(err).NotTo(HaveOccurred())

		repo = db.NewRepositoryRepository(database)
	})

	Describe("Find", func() {
		It("returns a matching repository", func() {
			err := repo.Create(&db.Repository{
				Name:  "my-special-repo",
				Owner: "my-special-owner",
			})

			Expect(err).NotTo(HaveOccurred())

			repository, found, err := repo.Find("my-special-owner", "my-special-repo")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(repository.Owner).To(Equal("my-special-owner"))
			Expect(repository.Name).To(Equal("my-special-repo"))
		})

		It("does not return an error when a repository is not found", func() {
			_, found, err := repo.Find("my-special-owner", "my-special-repo")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())
		})

		It("returns an error when the database blows up", func() {
			database.Close()

			_, found, err := repo.Find("my-special-owner", "my-special-repo")
			Expect(err).To(HaveOccurred())
			Expect(found).To(BeFalse())
		})
	})

	Describe("MustFind", func() {
		It("returns a matching repository", func() {
			err := repo.Create(&db.Repository{
				Name:  "my-special-repo",
				Owner: "my-special-owner",
			})

			Expect(err).NotTo(HaveOccurred())

			repository, err := repo.MustFind("my-special-owner", "my-special-repo")
			Expect(err).NotTo(HaveOccurred())

			Expect(repository.Owner).To(Equal("my-special-owner"))
			Expect(repository.Name).To(Equal("my-special-repo"))
		})

		It("returns an error when a repository is not found", func() {
			_, err := repo.MustFind("my-special-owner", "my-special-repo")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("record not found"))
		})

		It("returns an error when the database blows up", func() {
			database.Close()

			_, err := repo.MustFind("my-special-owner", "my-special-repo")
			Expect(err).To(HaveOccurred())
			Expect(err).NotTo(MatchError("record not found"))
		})
	})

	Describe("Create", func() {
		var (
			repository *db.Repository
		)

		BeforeEach(func() {
			repository = &db.Repository{
				Name:          "repo-name",
				Owner:         "owner-name",
				Path:          "path-to-repo-on-disk",
				SSHURL:        "repo-ssh-url",
				Private:       true,
				DefaultBranch: "master",
			}
		})

		It("saves the repository to the database", func() {
			err := repo.Create(repository)
			Expect(err).NotTo(HaveOccurred())

			savedRepository := db.Repository{}
			database.Where("name = ? AND owner = ?", repository.Name, repository.Owner).Last(&savedRepository)

			Expect(savedRepository.Name).To(Equal("repo-name"))
			Expect(savedRepository.Owner).To(Equal("owner-name"))
			Expect(savedRepository.Path).To(Equal("path-to-repo-on-disk"))
			Expect(savedRepository.SSHURL).To(Equal("repo-ssh-url"))
			Expect(savedRepository.Private).To(BeTrue())
			Expect(savedRepository.DefaultBranch).To(Equal("master"))
		})
	})

	Describe("MarkAsCloned", func() {
		var repository *db.Repository

		BeforeEach(func() {
			repository = &db.Repository{
				Name:          "some-repo",
				Owner:         "some-owner",
				SSHURL:        "some-url",
				Private:       true,
				DefaultBranch: "some-branch",
			}
			err := repo.Create(repository)
			Expect(err).NotTo(HaveOccurred())
		})

		It("marks the repo as cloned", func() {
			err := repo.MarkAsCloned("some-owner", "some-repo", "some-path")
			Expect(err).NotTo(HaveOccurred())

			savedRepository := &db.Repository{}
			database.Where("name = ? AND owner = ?", repository.Name, repository.Owner).Last(&savedRepository)

			Expect(savedRepository.Cloned).To(BeTrue())
		})

		It("updates the path on the repo", func() {
			err := repo.MarkAsCloned("some-owner", "some-repo", "some-path")
			Expect(err).NotTo(HaveOccurred())

			savedRepository := &db.Repository{}
			database.Where("name = ? AND owner = ?", repository.Name, repository.Owner).Last(&savedRepository)

			Expect(savedRepository.Path).To(Equal("some-path"))
		})
	})

	Describe("Reenable", func() {
		var repository *db.Repository

		BeforeEach(func() {
			repository = &db.Repository{
				Name:          "some-repo",
				Owner:         "some-owner",
				SSHURL:        "some-url",
				Private:       true,
				DefaultBranch: "some-branch",
				Disabled:      true,
			}
			err := repo.Create(repository)
			Expect(err).NotTo(HaveOccurred())
		})

		It("marks the repo as enabled", func() {
			err := repo.Reenable("some-owner", "some-repo")
			Expect(err).NotTo(HaveOccurred())

			savedRepository := &db.Repository{}
			database.Where("name = ? AND owner = ?", repository.Name, repository.Owner).Last(&savedRepository)

			Expect(savedRepository.Disabled).To(BeFalse())
		})
	})

	Describe("Active", func() {
		var (
			savedRepository db.Repository
		)

		BeforeEach(func() {
			repository := &db.Repository{
				Name:          "some-repo",
				Owner:         "some-owner",
				SSHURL:        "some-url",
				Private:       true,
				DefaultBranch: "some-branch",
				Cloned:        true,
			}
			err := repo.Create(repository)
			Expect(err).NotTo(HaveOccurred())

			err = database.Where("name = ? AND owner = ?", repository.Name, repository.Owner).
				Last(&savedRepository).Error
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the repository is enabled and cloned", func() {
			It("does not return the repository", func() {
				repos, err := repo.Active()
				Expect(err).NotTo(HaveOccurred())
				Expect(repos).To(ConsistOf(savedRepository))
			})
		})

		Context("when the repository is disabled", func() {
			BeforeEach(func() {
				_, err := database.DB().Exec(`UPDATE repositories SET disabled = true WHERE id = ?`, savedRepository.ID)
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not return the repository", func() {
				repos, err := repo.Active()
				Expect(err).NotTo(HaveOccurred())
				Expect(repos).To(BeEmpty())
			})
		})

		Context("when the repository has not been cloned", func() {
			BeforeEach(func() {
				err := database.Model(&db.Repository{}).Where("id = ?", savedRepository.ID).Update("cloned", false).Error
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not return the repository", func() {
				repos, err := repo.Active()
				Expect(err).NotTo(HaveOccurred())

				Expect(repos).To(BeEmpty())
			})
		})
	})

	Describe("NotScannedWithVersion", func() {
		var repository *db.Repository

		BeforeEach(func() {
			repository = &db.Repository{
				Name:          "some-repo",
				Owner:         "some-owner",
				SSHURL:        "some-url",
				Private:       true,
				DefaultBranch: "some-branch",
				Cloned:        true,
			}
			err := repo.Create(repository)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns an empty slice", func() {
			repos, err := repo.NotScannedWithVersion(42)
			Expect(err).NotTo(HaveOccurred())
			Expect(repos).To(BeEmpty())
		})

		Context("when the repository has scans for the specified version", func() {
			BeforeEach(func() {
				err := database.Create(&db.Scan{
					RepositoryID: &repository.ID,
				}).Error
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns a slice with the repository", func() {
				repos, err := repo.NotScannedWithVersion(42)
				Expect(err).NotTo(HaveOccurred())
				Expect(repos).To(HaveLen(1))
				Expect(repos[0].Name).To(Equal("some-repo"))
				Expect(repos[0].Owner).To(Equal("some-owner"))
			})
		})
	})

	Describe("RegisterFailedFetch", func() {
		var repository *db.Repository

		BeforeEach(func() {
			repository = &db.Repository{
				Name:          "some-repo",
				Owner:         "some-owner",
				SSHURL:        "some-url",
				Private:       true,
				DefaultBranch: "some-branch",
				Cloned:        true,
				FailedFetches: db.FailedFetchThreshold - 2,
			}
		})

		JustBeforeEach(func() {
			err := repo.Create(repository)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the repository has failed less than a threshold", func() {
			BeforeEach(func() {
				repository.FailedFetches = db.FailedFetchThreshold - 2
			})

			It("increments the failed fetch threshold", func() {
				err := repo.RegisterFailedFetch(logger, repository)
				Expect(err).NotTo(HaveOccurred())

				var failedFetches int
				err = database.DB().QueryRow(`
					SELECT failed_fetches
					FROM repositories
					WHERE id = ?
				`, repository.ID).Scan(&failedFetches)
				Expect(err).NotTo(HaveOccurred())

				Expect(failedFetches).To(Equal(db.FailedFetchThreshold - 1))
			})

			It("does not mark the repository as disabled", func() {
				err := repo.RegisterFailedFetch(logger, repository)
				Expect(err).NotTo(HaveOccurred())

				var disabled bool
				err = database.DB().QueryRow(`
					SELECT disabled
					FROM repositories
					WHERE id = ?
				`, repository.ID).Scan(&disabled)
				Expect(err).NotTo(HaveOccurred())

				Expect(disabled).To(BeFalse())
			})
		})

		Context("when the repository failing causes it to hit the threshold", func() {
			BeforeEach(func() {
				repository.FailedFetches = db.FailedFetchThreshold - 1
			})

			It("marks the repository as disabled", func() {
				err := repo.RegisterFailedFetch(logger, repository)
				Expect(err).NotTo(HaveOccurred())

				var disabled bool
				err = database.DB().QueryRow(`
					SELECT disabled
					FROM repositories
					WHERE id = ?
				`, repository.ID).Scan(&disabled)
				Expect(err).NotTo(HaveOccurred())

				Expect(disabled).To(BeTrue())
			})
		})

		It("returns an error when the repository can't be found", func() {
			err := repo.RegisterFailedFetch(logger, &db.Repository{
				Model: db.Model{
					ID: 1337,
				},
				Name:  "bad-repo",
				Owner: "bad-owner",
			})
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Update", func() {
		BeforeEach(func() {
			repository := &db.Repository{
				Name:          "repo-name",
				Owner:         "owner-name",
				SSHURL:        "repo-ssh-url",
				Private:       true,
				DefaultBranch: "master",
			}

			err := repo.Create(repository)
			Expect(err).NotTo(HaveOccurred())
		})

		It("updates a repository", func() {
			repository := &db.Repository{
				Name:          "repo-name",
				Owner:         "owner-name",
				SSHURL:        "new-ssh-url",
				Private:       false,
				DefaultBranch: "some-branch",
			}

			err := repo.Update(repository)
			Expect(err).NotTo(HaveOccurred())

			updatedRepo, err := repo.MustFind("owner-name", "repo-name")
			Expect(err).NotTo(HaveOccurred())

			Expect(updatedRepo.SSHURL).To(Equal(repository.SSHURL))
			Expect(updatedRepo.Private).To(Equal(repository.Private))
			Expect(updatedRepo.DefaultBranch).To(Equal(repository.DefaultBranch))
		})
	})
})

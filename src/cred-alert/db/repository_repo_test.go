package db_test

import (
	"cred-alert/db"
	"encoding/json"
	"time"

	"github.com/jinzhu/gorm"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RepositoryRepo", func() {
	var (
		repo     db.RepositoryRepository
		database *gorm.DB
	)

	BeforeEach(func() {
		var err error
		database, err = dbRunner.GormDB()
		Expect(err).NotTo(HaveOccurred())

		repo = db.NewRepositoryRepository(database)
	})

	Describe("FindOrCreate", func() {
		var (
			rawJSON      map[string]interface{}
			rawJSONBytes []byte
			repository   *db.Repository
		)

		BeforeEach(func() {
			rawJSON = map[string]interface{}{
				"path": "path-to-repo-on-disk",
				"name": "repo-name",
				"owner": map[string]interface{}{
					"login": "owner-name",
				},
				"private":        true,
				"default_branch": "master",
			}

			var err error
			rawJSONBytes, err = json.Marshal(rawJSON)
			Expect(err).NotTo(HaveOccurred())

			repository = &db.Repository{
				Name:          "repo-name",
				Owner:         "owner-name",
				Path:          "path-to-repo-on-disk",
				SSHURL:        "repo-ssh-url",
				Private:       true,
				DefaultBranch: "master",
				RawJSON:       rawJSONBytes,
			}
		})

		It("saves the repository to the database", func() {
			err := repo.FindOrCreate(repository)
			Expect(err).NotTo(HaveOccurred())

			savedRepository := &db.Repository{}
			database.Where("name = ? AND owner = ?", repository.Name, repository.Owner).Last(&savedRepository)

			Expect(savedRepository.Name).To(Equal("repo-name"))
			Expect(savedRepository.Owner).To(Equal("owner-name"))
			Expect(savedRepository.Path).To(Equal("path-to-repo-on-disk"))
			Expect(savedRepository.SSHURL).To(Equal("repo-ssh-url"))
			Expect(savedRepository.Private).To(BeTrue())
			Expect(savedRepository.DefaultBranch).To(Equal("master"))

			var actualRaw map[string]interface{}
			err = json.Unmarshal(savedRepository.RawJSON, &actualRaw)
			Expect(err).NotTo(HaveOccurred())

			Expect(actualRaw).To(Equal(rawJSON))
		})

		Context("when a repo with the same name and owner exists", func() {
			BeforeEach(func() {
				err := repo.FindOrCreate(&db.Repository{
					Name:          "repo-name",
					Owner:         "owner-name",
					Path:          "path-to-repo-on-disk",
					SSHURL:        "repo-ssh-url",
					Private:       true,
					DefaultBranch: "master",
					RawJSON:       rawJSONBytes,
				})
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns the saved repository", func() {
				err := repo.FindOrCreate(&db.Repository{
					Name:          "repo-name",
					Owner:         "owner-name",
					Path:          "path-to-repo-on-disk",
					SSHURL:        "repo-ssh-url",
					Private:       true,
					DefaultBranch: "master",
					RawJSON:       rawJSONBytes,
				})
				Expect(err).NotTo(HaveOccurred())

				var count int
				database.Model(&db.Repository{}).Where(
					"name = ? AND owner = ?", repository.Name, repository.Owner,
				).Count(&count)
				Expect(count).To(Equal(1))
			})
		})
	})

	Describe("FindOrCreate", func() {
		var (
			rawJSON      map[string]interface{}
			rawJSONBytes []byte
			repository   *db.Repository
		)

		BeforeEach(func() {
			rawJSON = map[string]interface{}{
				"path": "path-to-repo-on-disk",
				"name": "repo-name",
				"owner": map[string]interface{}{
					"login": "owner-name",
				},
				"private":        true,
				"default_branch": "master",
			}

			var err error
			rawJSONBytes, err = json.Marshal(rawJSON)
			Expect(err).NotTo(HaveOccurred())

			repository = &db.Repository{
				Name:          "repo-name",
				Owner:         "owner-name",
				Path:          "path-to-repo-on-disk",
				SSHURL:        "repo-ssh-url",
				Private:       true,
				DefaultBranch: "master",
				RawJSON:       rawJSONBytes,
			}
		})

		It("saves the repository to the database", func() {
			err := repo.Create(repository)
			Expect(err).NotTo(HaveOccurred())

			savedRepository := &db.Repository{}
			database.Where("name = ? AND owner = ?", repository.Name, repository.Owner).Last(&savedRepository)

			Expect(savedRepository.Name).To(Equal("repo-name"))
			Expect(savedRepository.Owner).To(Equal("owner-name"))
			Expect(savedRepository.Path).To(Equal("path-to-repo-on-disk"))
			Expect(savedRepository.SSHURL).To(Equal("repo-ssh-url"))
			Expect(savedRepository.Private).To(BeTrue())
			Expect(savedRepository.DefaultBranch).To(Equal("master"))

			var actualRaw map[string]interface{}
			err = json.Unmarshal(savedRepository.RawJSON, &actualRaw)
			Expect(err).NotTo(HaveOccurred())

			Expect(actualRaw).To(Equal(rawJSON))
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
				RawJSON:       []byte("some-json"),
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

	Describe("NotFetchedSince", func() {
		var (
			savedFetch      db.Fetch
			savedRepository db.Repository
		)

		BeforeEach(func() {
			repository := &db.Repository{
				Name:          "some-repo",
				Owner:         "some-owner",
				SSHURL:        "some-url",
				Private:       true,
				DefaultBranch: "some-branch",
				RawJSON:       []byte("some-json"),
				Cloned:        true,
			}
			err := repo.Create(repository)
			Expect(err).NotTo(HaveOccurred())

			database.Where("name = ? AND owner = ?", repository.Name, repository.Owner).Last(&savedRepository)

			err = database.Model(&db.Fetch{}).Create(&db.Fetch{
				RepositoryID: savedRepository.ID,
				Changes:      []byte("changes"),
			}).Error
			Expect(err).NotTo(HaveOccurred())

			database.Where("repository_id = ?", repository.ID).Last(&savedFetch)
		})

		Context("when the repo's latest fetch is later than the given time", func() {
			BeforeEach(func() {
				t := time.Now().Add(-5 * time.Minute)
				database.Model(&db.Fetch{}).Where("id = ?", savedFetch.ID).Update("created_at", t)
			})

			It("does not return the repository", func() {
				repos, err := repo.NotFetchedSince(time.Now().Add(-10 * time.Minute))
				Expect(err).NotTo(HaveOccurred())
				Expect(repos).To(BeEmpty())
			})
		})

		Context("when the repo's latest fetch is not later than the given time", func() {
			BeforeEach(func() {
				t := time.Now().Add(-15 * time.Minute)
				database.Model(&db.Fetch{}).Where("id = ?", savedFetch.ID).Update("created_at", t)
			})

			It("returns the repo", func() {
				repos, err := repo.NotFetchedSince(time.Now().UTC().Add(-10 * time.Minute))
				Expect(err).NotTo(HaveOccurred())
				Expect(repos).To(ConsistOf(savedRepository))
			})

			Context("when the repository has not been cloned", func() {
				BeforeEach(func() {
					err := database.Model(&db.Repository{}).Where("id = ?", savedRepository.ID).Update("cloned", false).Error
					Expect(err).NotTo(HaveOccurred())
				})

				It("does not return the repository", func() {
					repos, err := repo.NotFetchedSince(time.Now().Add(-10 * time.Minute))
					Expect(err).NotTo(HaveOccurred())
					Expect(repos).To(BeEmpty())
				})
			})
		})
	})
})

package db_test

import (
	"cred-alert/db"
	"encoding/json"

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
})

package db_test

import (
	"cred-alert/db"
	"encoding/json"
	"time"

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

	Describe("Create", func() {
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

	Describe("DueForFetch", func() {
		var (
			savedFetch      db.Fetch
			savedRepository db.Repository
		)

		BeforeEach(func() {
			repository := &db.Repository{
				Name:                 "some-repo",
				Owner:                "some-owner",
				SSHURL:               "some-url",
				Private:              true,
				DefaultBranch:        "some-branch",
				RawJSON:              []byte("some-json"),
				Cloned:               true,
				FetchIntervalSeconds: 8 * 60 * 60,
			}
			err := repo.Create(repository)
			Expect(err).NotTo(HaveOccurred())

			err = database.
				Where("name = ? AND owner = ?", repository.Name, repository.Owner).
				Last(&savedRepository).Error
			Expect(err).NotTo(HaveOccurred())

			err = database.Model(&db.Fetch{}).Create(&db.Fetch{
				RepositoryID: savedRepository.ID,
				Changes:      []byte("changes"),
			}).Error
			Expect(err).NotTo(HaveOccurred())

			database.Where("repository_id = ?", repository.ID).Last(&savedFetch)
		})

		Context("when the time between the last fetch and now is shorter than the fetch interval", func() {
			BeforeEach(func() {
				t := time.Now().Add(-15 * time.Minute)
				database.Model(&db.Fetch{}).Where("id = ?", savedFetch.ID).Update("created_at", t)
			})

			It("does not return that repository", func() {
				repos, err := repo.DueForFetch()
				Expect(err).NotTo(HaveOccurred())
				Expect(repos).To(BeEmpty())
			})
		})

		Context("when time between the last fetch and now is longer than the fetch interval", func() {
			BeforeEach(func() {
				t := time.Now().Add(-10 * time.Hour)
				database.Model(&db.Fetch{}).Where("id = ?", savedFetch.ID).Update("created_at", t)
			})

			It("returns a list of repositories that are due for fetch", func() {
				repos, err := repo.DueForFetch()
				Expect(err).NotTo(HaveOccurred())
				Expect(repos).To(HaveLen(1))
				Expect(repos).To(ConsistOf(savedRepository))
			})
		})

		Context("when the repository has never been fetched", func() {
			var (
				neverFetchedRepo db.Repository
			)

			BeforeEach(func() {
				repository := &db.Repository{
					Name:                 "some-unfetched-repo",
					Owner:                "some-unfetched-owner",
					SSHURL:               "some-unfetched-url",
					Private:              true,
					DefaultBranch:        "some-branch",
					RawJSON:              []byte("some-json"),
					Cloned:               true,
					FetchIntervalSeconds: 8 * 60 * 60,
				}
				err := repo.Create(repository)
				Expect(err).NotTo(HaveOccurred())

				err = database.Where("name = ? AND owner = ?", repository.Name, repository.Owner).Last(&neverFetchedRepo).Error
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns the repository", func() {
				repos, err := repo.DueForFetch()
				Expect(err).NotTo(HaveOccurred())
				Expect(repos).To(HaveLen(1))
				Expect(repos).To(ConsistOf(neverFetchedRepo))
			})
		})

		Context("when the repository is disabled", func() {
			BeforeEach(func() {
				_, err := database.DB().Exec(`UPDATE repositories SET disabled = true WHERE id = ?`, savedRepository.ID)
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not return the repository", func() {
				repos, err := repo.DueForFetch()
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
				repos, err := repo.DueForFetch()
				Expect(err).NotTo(HaveOccurred())

				Expect(repos).To(BeEmpty())
			})
		})
	})

	Describe("Active", func() {
		var (
			savedRepository db.Repository
		)

		BeforeEach(func() {
			repository := &db.Repository{
				Name:                 "some-repo",
				Owner:                "some-owner",
				SSHURL:               "some-url",
				Private:              true,
				DefaultBranch:        "some-branch",
				RawJSON:              []byte("some-json"),
				Cloned:               true,
				FetchIntervalSeconds: 8 * 60 * 60,
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

	Describe("LastActivity", func() {
		var (
			repository *db.Repository
			fetchedAt  time.Time
		)

		BeforeEach(func() {
			repository = &db.Repository{
				Name:                 "some-unfetched-repo",
				Owner:                "some-unfetched-owner",
				SSHURL:               "some-unfetched-url",
				Private:              true,
				DefaultBranch:        "some-branch",
				RawJSON:              []byte("some-json"),
				Cloned:               true,
				FetchIntervalSeconds: 6 * 60 * 60,
			}
			err := repo.Create(repository)
			Expect(err).NotTo(HaveOccurred())

			fetchedAt = time.Now()
		})

		Context("when there are fetches with changes", func() {
			BeforeEach(func() {
				fetch1 := &db.Fetch{
					RepositoryID: repository.ID,
					Changes:      []byte("changes"),
					Model: db.Model{
						CreatedAt: fetchedAt,
					},
				}

				err := database.Model(&db.Fetch{}).Create(fetch1).Error
				Expect(err).NotTo(HaveOccurred())

				fetch2 := &db.Fetch{
					RepositoryID: repository.ID,
					Changes:      []byte("{}"),
					Model: db.Model{
						CreatedAt: fetchedAt.Add(time.Second),
					},
				}

				err = database.Model(&db.Fetch{}).Create(fetch2).Error
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns the time of last activity", func() {
				lastActivity, err := repo.LastActivity(repository)
				Expect(err).NotTo(HaveOccurred())
				Expect(lastActivity).To(BeTemporally("~", fetchedAt, time.Second))
			})
		})

		Context("when there are no fetches with changes", func() {
			BeforeEach(func() {
				fetch := &db.Fetch{
					RepositoryID: repository.ID,
					Changes:      []byte("{}"),
					Model: db.Model{
						CreatedAt: fetchedAt,
					},
				}

				err := database.Model(&db.Fetch{}).Create(fetch).Error
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns an error", func() {
				_, err := repo.LastActivity(repository)
				Expect(err).To(Equal(db.NoChangesError))
			})
		})

		Context("when there are no fetches", func() {
			It("returns an error", func() {
				_, err := repo.LastActivity(repository)
				Expect(err).To(Equal(db.NeverBeenFetchedError))
			})
		})
	})

	Describe("UpdateFetchInterval", func() {
		var (
			repository *db.Repository
		)

		BeforeEach(func() {
			repository = &db.Repository{
				Name:                 "some-unfetched-repo",
				Owner:                "some-unfetched-owner",
				SSHURL:               "some-unfetched-url",
				Private:              true,
				DefaultBranch:        "some-branch",
				RawJSON:              []byte("some-json"),
				Cloned:               true,
				FetchIntervalSeconds: 6 * 60 * 60,
			}
			err := repo.Create(repository)
			Expect(err).NotTo(HaveOccurred())
		})

		It("updates fetch interval", func() {
			err := repo.UpdateFetchInterval(repository, 8*time.Hour)
			Expect(err).NotTo(HaveOccurred())

			var sec int
			err = database.DB().QueryRow(`
				SELECT fetch_interval
				FROM repositories
				WHERE id = ?
				`, repository.ID).Scan(&sec)
			Expect(err).NotTo(HaveOccurred())

			Expect(sec).To(Equal(8 * 60 * 60))
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
				RawJSON:       []byte("some-json"),
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
				RawJSON:       []byte("some-json"),
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
				Name:    "bad-repo",
				Owner:   "bad-owner",
				RawJSON: []byte("bad-json"),
			})
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("UpdateCredentialCount", func() {
		var repository *db.Repository

		BeforeEach(func() {
			repository = &db.Repository{
				Name:          "some-repo",
				Owner:         "some-owner",
				SSHURL:        "some-url",
				DefaultBranch: "some-branch",
				RawJSON:       []byte("some-json"),
				Cloned:        true,
			}
			err := repo.Create(repository)
			Expect(err).NotTo(HaveOccurred())
		})

		It("updates the credential count on the repository", func() {
			expectedCredentialCounts := map[string]uint{
				"some-branch":       42,
				"some-other-branch": 84,
			}

			err := repo.UpdateCredentialCount(repository, expectedCredentialCounts)
			Expect(err).NotTo(HaveOccurred())

			var counts []byte
			err = database.DB().QueryRow(`
				SELECT credential_counts
				FROM repositories
				WHERE id = ?
			`, repository.ID).Scan(&counts)
			Expect(err).NotTo(HaveOccurred())

			actualCredentialCounts := make(map[string]interface{})
			err = json.Unmarshal(counts, &actualCredentialCounts)
			Expect(err).NotTo(HaveOccurred())
			for k, v := range actualCredentialCounts {
				Expect(v).To(BeNumerically("==", expectedCredentialCounts[k]))
			}
		})
	})
})

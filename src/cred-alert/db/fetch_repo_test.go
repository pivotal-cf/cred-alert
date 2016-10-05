package db_test

import (
	"cred-alert/db"
	"encoding/json"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/jinzhu/gorm"
	git "github.com/libgit2/git2go"
	uuid "github.com/satori/go.uuid"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("FetchRepo", func() {
	var (
		repo     db.FetchRepository
		database *gorm.DB
		logger   *lagertest.TestLogger
	)

	BeforeEach(func() {
		var err error
		database, err = dbRunner.GormDB()
		Expect(err).NotTo(HaveOccurred())

		logger = lagertest.NewTestLogger("repository-repository-test")
		repo = db.NewFetchRepository(database)
	})

	Describe("Save", func() {
		var (
			a       *git.Oid
			b       *git.Oid
			c       *git.Oid
			d       *git.Oid
			changes map[string][]string
		)

		BeforeEach(func() {
			var err error
			a, err = git.NewOid("fce98866a7d559757a0a501aa548e244a46ad00a")
			Expect(err).NotTo(HaveOccurred())
			b, err = git.NewOid("3f5c0cc6c73ddb1a3aa05725c48ca1223367fb74")
			Expect(err).NotTo(HaveOccurred())
			c, err = git.NewOid("7257894438275f68380aa6d75015ef7a0ca6757b")
			Expect(err).NotTo(HaveOccurred())
			d, err = git.NewOid("53fc72ccf2ef176a02169aeebf5c8427861e9b0e")
			Expect(err).NotTo(HaveOccurred())

			changes = map[string][]string{
				"refs/remotes/origin/master":  {a.String(), b.String()},
				"refs/remotes/origin/develop": {c.String(), d.String()},
			}
		})

		It("saves the fetch to the database", func() {
			bs, err := json.Marshal(changes)
			Expect(err).NotTo(HaveOccurred())

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

			repository := &db.Repository{
				Name:          uuid.NewV4().String(),
				Owner:         "owner-name",
				Path:          "path-to-repo-on-disk",
				SSHURL:        "repo-ssh-url",
				Private:       true,
				DefaultBranch: "master",
				RawJSON:       repoJSONBytes,
			}

			err = repo.RegisterFetch(logger, &db.Fetch{
				Repository: *repository,
				Path:       "path-to-repo-on-disk",
				Changes:    bs,
			})
			Expect(err).NotTo(HaveOccurred())

			savedFetch := &db.Fetch{}
			database.Last(&savedFetch)
			Expect(savedFetch.Path).To(Equal("path-to-repo-on-disk"))

			var actualChanges map[string][]string
			err = json.Unmarshal(savedFetch.Changes, &actualChanges)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualChanges).To(Equal(changes))
		})
	})
})

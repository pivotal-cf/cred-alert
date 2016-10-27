package db_test

import (
	"cred-alert/db"
	"database/sql"

	"github.com/jinzhu/gorm"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CredentialRepo", func() {
	var (
		repo   db.CredentialRepository
		gormDB *gorm.DB
		sqlDB  *sql.DB
	)

	BeforeEach(func() {
		var err error
		gormDB, err = dbRunner.GormDB()
		Expect(err).NotTo(HaveOccurred())
		sqlDB = gormDB.DB()

		repo = db.NewCredentialRepository(gormDB)
	})

	Describe("UniqueSHAsForRepoAndRulesVersion", func() {
		var repositoryID int64

		BeforeEach(func() {
			result, err := sqlDB.Exec(`
				INSERT INTO repositories (name, owner, path, ssh_url, default_branch, raw_json, created_at, updated_at)
				VALUES (
					'some-repo',
					'some-owner',
					'some-path',
					'some-url',
					'some-branch',
					'{}',
					NOW(),
					NOW()
				)`)
			Expect(err).NotTo(HaveOccurred())
			rowsAffected, err := result.RowsAffected()
			Expect(err).NotTo(HaveOccurred())
			Expect(rowsAffected).To(BeNumerically("==", 1))

			repositoryID, err = result.LastInsertId()
			Expect(err).NotTo(HaveOccurred())

			result, err = sqlDB.Exec(`
				INSERT INTO scans (type, rules_version, repository_id, created_at, updated_at)
				VALUES (
					'some-type',
					4,
					?,
					NOW(),
					NOW()
				)`, repositoryID)
			Expect(err).NotTo(HaveOccurred())
			rowsAffected, err = result.RowsAffected()
			Expect(err).NotTo(HaveOccurred())
			Expect(rowsAffected).To(BeNumerically("==", 1))

			scanID, err := result.LastInsertId()
			Expect(err).NotTo(HaveOccurred())

			result, err = sqlDB.Exec(`
				INSERT INTO credentials (scan_id, owner, repository, sha, created_at, updated_at)
				VALUES (
					?,
					'some-owner',
					'some-repository',
					'matching-sha',
					NOW(),
					NOW()
				)`, scanID)
			Expect(err).NotTo(HaveOccurred())
			rowsAffected, err = result.RowsAffected()
			Expect(err).NotTo(HaveOccurred())
			Expect(rowsAffected).To(BeNumerically("==", 1))

			result, err = sqlDB.Exec(`
				INSERT INTO credentials (scan_id, owner, repository, sha, created_at, updated_at)
				VALUES (
					?,
					'some-owner',
					'some-repository',
					'matching-sha',
					NOW(),
					NOW()
				)`, scanID)
			Expect(err).NotTo(HaveOccurred())
			rowsAffected, err = result.RowsAffected()
			Expect(err).NotTo(HaveOccurred())
			Expect(rowsAffected).To(BeNumerically("==", 1))

			result, err = sqlDB.Exec(`
				INSERT INTO credentials (scan_id, owner, repository, sha, created_at, updated_at)
				VALUES (
					?,
					'some-owner',
					'some-repository',
					'another-matching-sha',
					NOW(),
					NOW()
				)`, scanID)
			Expect(err).NotTo(HaveOccurred())
			rowsAffected, err = result.RowsAffected()
			Expect(err).NotTo(HaveOccurred())
			Expect(rowsAffected).To(BeNumerically("==", 1))

			result, err = sqlDB.Exec(`
				INSERT INTO scans (type, rules_version, repository_id, created_at, updated_at)
				VALUES (
					'some-type',
					5,
					?,
					NOW(),
					NOW()
				)`, repositoryID)
			Expect(err).NotTo(HaveOccurred())
			rowsAffected, err = result.RowsAffected()
			Expect(err).NotTo(HaveOccurred())
			Expect(rowsAffected).To(BeNumerically("==", 1))

			otherScanID, err := result.LastInsertId()
			Expect(err).NotTo(HaveOccurred())

			result, err = sqlDB.Exec(`
				INSERT INTO credentials (scan_id, owner, repository, sha, created_at, updated_at)
				VALUES (
					?,
					'some-owner',
					'some-repository',
					'non-matching-sha',
					NOW(),
					NOW()
				)`, otherScanID)
			Expect(err).NotTo(HaveOccurred())
			rowsAffected, err = result.RowsAffected()
			Expect(err).NotTo(HaveOccurred())
			Expect(rowsAffected).To(BeNumerically("==", 1))
		})

		It("returns unique SHAs of credentials for scans made with the given rules version for the given repository", func() {
			dbRepository := db.Repository{
				Model: db.Model{
					ID: uint(repositoryID),
				},
			}
			shas, err := repo.UniqueSHAsForRepoAndRulesVersion(dbRepository, 4)
			Expect(err).NotTo(HaveOccurred())
			Expect(shas).To(Equal([]string{"matching-sha", "another-matching-sha"}))
		})
	})
})

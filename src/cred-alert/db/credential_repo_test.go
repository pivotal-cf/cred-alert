package db_test

import (
	"cred-alert/db"
	"database/sql"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/jinzhu/gorm"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CredentialRepo", func() {
	var (
		clock    *fakeclock.FakeClock
		repo     db.CredentialRepository
		scanRepo db.ScanRepository
		gormDB   *gorm.DB
		sqlDB    *sql.DB
	)

	BeforeEach(func() {
		var err error
		gormDB, err = dbRunner.GormDB()
		Expect(err).NotTo(HaveOccurred())
		sqlDB = gormDB.DB()

		repo = db.NewCredentialRepository(gormDB)
		scanRepo = db.NewScanRepository(gormDB, clock)
	})

	Describe("UniqueSHAsForRepoAndRulesVersion", func() {
		var repositoryID int64

		BeforeEach(func() {
			result, err := sqlDB.Exec(`
				INSERT INTO repositories (name, owner, path, ssh_url, default_branch, created_at, updated_at)
				VALUES (
					'some-repo',
					'some-owner',
					'some-path',
					'some-url',
					'some-branch',
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
	Describe("CredentialReported", func() {
		var expectedScanID int
		var repositoryID uint
		BeforeEach(func() {
			//Insert repository
			_, err := gormDB.DB().Exec(`
					INSERT INTO repositories (name, owner)
					VALUES (
						'some-repo',
						'some-owner'
					)
				`)
			Expect(err).NotTo(HaveOccurred())
			err = gormDB.DB().QueryRow(`SELECT id FROM repositories ORDER BY id DESC LIMIT 1`).Scan(&repositoryID)
			Expect(err).NotTo(HaveOccurred())

			repository := &db.Repository{
				Model: db.Model{
					ID: repositoryID,
				},
			}

			//Insert scan
			logger := lagertest.NewTestLogger("commit-repository")
			err = scanRepo.Start(logger, "scan-type", "branch", "some-start-sha", "some-stop-sha", repository).Finish()
			Expect(err).NotTo(HaveOccurred())
			err = gormDB.DB().QueryRow(`SELECT id FROM scans ORDER BY id DESC LIMIT 1`).Scan(&expectedScanID)
			Expect(err).NotTo(HaveOccurred())

			//insert credential
			_, err = sqlDB.Exec(`
				INSERT INTO credentials(owner, repository, sha, path, line_number, match_start, match_end)
				VALUES (
					'some-owner',
					'some-repo',
					'some-sha',
					'some-path',
					1,
					2,
					3
				)`)

			Expect(err).NotTo(HaveOccurred())
		})
		FContext("when credential already reported", func() {
			It("should return true", func() {
				cred := db.Credential{
					ScanID:     uint(expectedScanID),
					Owner:      "some-owner",
					Repository: "some-repo",
					SHA:        "some-sha",
					Path:       "some-path",
					LineNumber: 1,
					MatchStart: 2,
					MatchEnd:   3,
				}
				exists, err := repo.CredentialReported(&cred)

				Expect(err).NotTo(HaveOccurred())
				Expect(exists).To(BeTrue())
			})
		})
		FContext("when credential not reported", func() {
			It("should return false", func() {
				cred := db.Credential{
					Owner:      "other-owner",
					Repository: "other-repository",
					SHA:        "other-sha",
					Path:       "other-path",
					LineNumber: 11,
					MatchStart: 22,
					MatchEnd:   33,
				}
				exists, err := repo.CredentialReported(&cred)

				Expect(err).NotTo(HaveOccurred())
				Expect(exists).To(BeFalse())
			})
		})
	})
})

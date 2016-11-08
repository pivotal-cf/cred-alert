package db_test

import (
	"cred-alert/db"
	"fmt"

	"github.com/jinzhu/gorm"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("StatsRepo", func() {
	var (
		database  *gorm.DB
		statsRepo db.StatsRepository
	)

	BeforeEach(func() {
		dbRunner.Truncate()

		var err error
		database, err = dbRunner.GormDB()
		Expect(err).NotTo(HaveOccurred())

		statsRepo = db.NewStatsRepository(database)
	})

	Describe("RepositoryCount", func() {
		BeforeEach(func() {
			for i := 0; i < 6; i++ {
				err := database.Create(&db.Repository{
					Name:    fmt.Sprintf("some-name-%d", i),
					Owner:   "some-owner",
					RawJSON: []byte("some-raw-json"),
				}).Error
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("returns the count of repositories", func() {
			Expect(statsRepo.RepositoryCount()).To(Equal(6))
		})
	})

	Describe("FetchCount", func() {
		BeforeEach(func() {
			repository := &db.Repository{
				Name:    "some-name",
				Owner:   "some-owner",
				RawJSON: []byte("some-raw-json"),
			}
			err := database.Create(repository).Error
			Expect(err).NotTo(HaveOccurred())

			for i := 0; i < 6; i++ {
				err := database.Create(&db.Fetch{
					Repository: *repository,
					Changes:    []byte("changes"),
				}).Error
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("returns the count of repositories", func() {
			Expect(statsRepo.FetchCount()).To(Equal(6))
		})
	})

	Describe("CredentialCount", func() {
		BeforeEach(func() {
			scan := &db.Scan{}
			err := database.Create(scan).Error
			Expect(err).NotTo(HaveOccurred())

			for i := 0; i < 6; i++ {
				err := database.Create(&db.Credential{
					Scan: *scan,
				}).Error
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("returns the count of repositories", func() {
			Expect(statsRepo.CredentialCount()).To(Equal(6))
		})
	})
})

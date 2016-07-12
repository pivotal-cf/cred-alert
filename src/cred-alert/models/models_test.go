package models_test

import (
	"io/ioutil"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cred-alert/models"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

var _ = Describe("Database Connections", func() {
	var (
		db *gorm.DB
	)

	BeforeEach(func() {
		dbFileHandle, err := ioutil.TempFile("", "test.db")
		Expect(err).NotTo(HaveOccurred())

		err = dbFileHandle.Close()
		Expect(err).NotTo(HaveOccurred())

		db, err = gorm.Open("sqlite3", dbFileHandle.Name())
		Expect(err).NotTo(HaveOccurred())
		db.AutoMigrate(&models.Repo{}, &models.Ref{}, &models.DiffScan{}, &models.Commit{})
	})

	Describe("auto-migrations", func() {
		It("creates the Repo table", func() {
			Expect(db.HasTable(&models.Repo{})).To(BeTrue())
		})

		It("creates the DiffScan table", func() {
			Expect(db.HasTable(&models.DiffScan{})).To(BeTrue())
		})

		It("creates the Ref table", func() {
			Expect(db.HasTable(&models.Ref{})).To(BeTrue())
		})

		It("creates the Commit table", func() {
			Expect(db.HasTable(&models.Commit{})).To(BeTrue())
		})
	})
})

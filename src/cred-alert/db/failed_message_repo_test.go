package db_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/jinzhu/gorm"

	"cred-alert/db"
)

var _ = Describe("Failed Message Repo", func() {
	var (
		repo     db.FailedMessageRepository
		database *gorm.DB
		logger   *lagertest.TestLogger
	)

	BeforeEach(func() {
		var err error
		database, err = dbRunner.GormDB()
		Expect(err).NotTo(HaveOccurred())

		logger = lagertest.NewTestLogger("failed-message-repository-test")
		repo = db.NewFailedMessageRepository(database)
	})

	Describe("registering a failed message", func() {
		It("registers it", func() {
			retries, err := repo.RegisterFailedMessage(logger, "message-id")
			Expect(err).NotTo(HaveOccurred())
			Expect(retries).To(Equal(1))

			failedMessages, err := repo.GetFailedMessages(logger)
			Expect(failedMessages).To(HaveLen(1))
			Expect(failedMessages[0].MessageID).To(Equal("message-id"))
			Expect(failedMessages[0].Retries).To(Equal(1))
		})

		Context("when the message has failed before", func() {
			BeforeEach(func() {
				_, err := repo.RegisterFailedMessage(logger, "message-id")
				Expect(err).NotTo(HaveOccurred())
			})

			It("registers it and increments the retries", func() {
				retries, err := repo.RegisterFailedMessage(logger, "message-id")
				Expect(err).NotTo(HaveOccurred())
				Expect(retries).To(Equal(2))

				failedMessages, err := repo.GetFailedMessages(logger)
				Expect(failedMessages).To(HaveLen(1))
				Expect(failedMessages[0].MessageID).To(Equal("message-id"))
				Expect(failedMessages[0].Retries).To(Equal(2))
			})
		})

		Context("when a different message has failed before", func() {
			BeforeEach(func() {
				_, err := repo.RegisterFailedMessage(logger, "other-message-id")
				Expect(err).NotTo(HaveOccurred())
			})

			It("registers a new message", func() {
				retries, err := repo.RegisterFailedMessage(logger, "message-id")
				Expect(err).NotTo(HaveOccurred())
				Expect(retries).To(Equal(1))

				failedMessages, err := repo.GetFailedMessages(logger)
				Expect(failedMessages).To(HaveLen(2))
			})
		})
	})

	Describe("removing a failed message", func() {
		It("removes the message", func() {
			_, err := repo.RegisterFailedMessage(logger, "message-id")
			Expect(err).NotTo(HaveOccurred())

			err = repo.RemoveFailedMessage(logger, "message-id")
			Expect(err).NotTo(HaveOccurred())

			failedMessages, err := repo.GetFailedMessages(logger)
			Expect(failedMessages).To(HaveLen(0))
		})
	})

	Describe("marking a dead letter", func() {
		It("marks the message", func() {
			_, err := repo.RegisterFailedMessage(logger, "message-id")
			Expect(err).NotTo(HaveOccurred())

			deadLetters, err := repo.GetDeadLetters(logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(deadLetters).To(HaveLen(0))

			err = repo.MarkFailedMessageAsDead(logger, "message-id")
			Expect(err).NotTo(HaveOccurred())

			deadLetters, err = repo.GetDeadLetters(logger)
			Expect(deadLetters).To(HaveLen(1))
			Expect(deadLetters[0].MessageID).To(Equal("message-id"))
		})
	})
})

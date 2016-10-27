package queue_test

import (
	"cred-alert/db"
	"cred-alert/db/dbfakes"
	"cred-alert/queue"
	"cred-alert/revok/revokfakes"
	"errors"

	"cloud.google.com/go/pubsub"

	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PushEventProcessor", func() {
	var (
		logger               *lagertest.TestLogger
		pushEventProcessor   queue.PubSubProcessor
		changeDiscoverer     *revokfakes.FakeChangeDiscoverer
		repositoryRepository *dbfakes.FakeRepositoryRepository

		message *pubsub.Message
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("ingestor")
		changeDiscoverer = &revokfakes.FakeChangeDiscoverer{}
		repositoryRepository = &dbfakes.FakeRepositoryRepository{}
		pushEventProcessor = queue.NewPushEventProcessor(logger, changeDiscoverer, repositoryRepository)
	})

	Context("when the payload is a valid JSON PushEventPlan", func() {
		BeforeEach(func() {
			task := queue.PushEventPlan{
				Owner:      "some-owner",
				Repository: "some-repo",
				From:       "from-sha",
				To:         "to-sha",
				Private:    true,
			}.Task("message-id")

			message = &pubsub.Message{
				Attributes: map[string]string{
					"id":   task.ID(),
					"type": task.Type(),
				},
				Data: []byte(task.Payload()),
			}
		})

		It("looks up the repository in the database", func() {
			pushEventProcessor.Process(message)
			Expect(repositoryRepository.FindCallCount()).To(Equal(1))
			owner, name := repositoryRepository.FindArgsForCall(0)
			Expect(owner).To(Equal("some-owner"))
			Expect(name).To(Equal("some-repo"))
		})

		Context("when the repository can be found in the database", func() {
			var (
				expectedRepository *db.Repository
			)

			BeforeEach(func() {
				expectedRepository = &db.Repository{
					Owner: "some-owner",
					Name:  "some-name",
				}

				repositoryRepository.FindReturns(*expectedRepository, nil)
			})

			It("tries to do a fetch", func() {
				pushEventProcessor.Process(message)
				Expect(changeDiscoverer.FetchCallCount()).To(Equal(1))
				_, actualRepository := changeDiscoverer.FetchArgsForCall(0)
				Expect(actualRepository).To(Equal(*expectedRepository))
			})

			Context("when the fetch succeeds", func() {
				BeforeEach(func() {
					changeDiscoverer.FetchReturns(nil)
				})

				It("does not retry or return an error", func() {
					retry, err := pushEventProcessor.Process(message)
					Expect(retry).To(BeFalse())
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("when the fetch fails", func() {
				BeforeEach(func() {
					changeDiscoverer.FetchReturns(errors.New("an-error"))
				})

				It("returns an error that can be retried", func() {
					retry, err := pushEventProcessor.Process(message)
					Expect(retry).To(BeTrue())
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Context("when the repository can not be found in the database", func() {
			BeforeEach(func() {
				repositoryRepository.FindReturns(db.Repository{}, errors.New("an-error"))
			})

			It("does not try to do a fetch", func() {
				pushEventProcessor.Process(message)
				Expect(changeDiscoverer.FetchCallCount()).To(BeZero())
			})

			It("returns an error that cannot be retried", func() {
				retry, err := pushEventProcessor.Process(message)
				Expect(retry).To(BeFalse())
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Context("when the payload is not valid JSON", func() {
		BeforeEach(func() {
			bs := []byte("some bad bytes")

			message = &pubsub.Message{
				Attributes: map[string]string{
					"id":   "some-id",
					"type": "some-type",
				},
				Data: bs,
			}
		})

		It("does not look up the repository in the database", func() {
			pushEventProcessor.Process(message)
			Expect(repositoryRepository.FindCallCount()).To(BeZero())
		})

		It("does not try to do a fetch", func() {
			pushEventProcessor.Process(message)
			Expect(changeDiscoverer.FetchCallCount()).To(BeZero())
		})

		It("returns an error that cannot be retried", func() {
			retry, err := pushEventProcessor.Process(message)
			Expect(retry).To(BeFalse())
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when the payload is a valid JSON for a PushEventPlan but is missing the repository", func() {
		BeforeEach(func() {
			bs := []byte(`{
				"owner":"some-owner"
			}`)

			message = &pubsub.Message{
				Attributes: map[string]string{
					"id":   "some-id",
					"type": "some-type",
				},
				Data: bs,
			}
		})

		It("does not look up the repository in the database", func() {
			pushEventProcessor.Process(message)
			Expect(repositoryRepository.FindCallCount()).To(BeZero())
		})

		It("does not try to do a fetch", func() {
			pushEventProcessor.Process(message)
			Expect(changeDiscoverer.FetchCallCount()).To(BeZero())
		})

		It("returns an unretryable error", func() {
			retry, err := pushEventProcessor.Process(message)
			Expect(retry).To(BeFalse())
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when the payload is a valid JSON for a PushEventPlan but is missing the owner", func() {
		BeforeEach(func() {
			bs := []byte(`{
				"repository":"some-repository"
			}`)

			message = &pubsub.Message{
				Attributes: map[string]string{
					"id":   "some-id",
					"type": "some-type",
				},
				Data: bs,
			}
		})

		It("does not look up the repository in the database", func() {
			pushEventProcessor.Process(message)
			Expect(repositoryRepository.FindCallCount()).To(BeZero())
		})

		It("does not try to do a fetch", func() {
			pushEventProcessor.Process(message)
			Expect(changeDiscoverer.FetchCallCount()).To(BeZero())
		})

		It("returns an unretryable error", func() {
			retry, err := pushEventProcessor.Process(message)
			Expect(retry).To(BeFalse())
			Expect(err).To(HaveOccurred())
		})
	})
})

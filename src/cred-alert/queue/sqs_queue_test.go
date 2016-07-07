package queue_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"

	"cred-alert/queue"
	"cred-alert/queue/queuefakes"
)

var _ = Describe("SQS Queue", func() {
	var (
		sqsQueue queue.Queue
		service  *queuefakes.FakeSQSAPI

		expectedQueueName string
	)

	BeforeEach(func() {
		service = &queuefakes.FakeSQSAPI{}

		expectedQueueName = "test"
	})

	Context("we can successfully get the queue url", func() {
		var (
			expectedQueueUrl string
		)

		BeforeEach(func() {
			expectedQueueUrl = "https://aws.example.com/1234567/queue-name"

			urlOutput := &sqs.GetQueueUrlOutput{
				QueueUrl: aws.String(expectedQueueUrl),
			}
			service.GetQueueUrlReturns(urlOutput, nil)
		})

		JustBeforeEach(func() {
			var err error
			sqsQueue, err = queue.BuildSQSQueue(service, expectedQueueName)
			Expect(err).ToNot(HaveOccurred())
		})

		Describe("sending work to the queue", func() {
			It("sends the correct message to SQS", func() {
				task := &queuefakes.FakeTask{}
				err := sqsQueue.Enqueue(task)
				Expect(err).NotTo(HaveOccurred())

				Expect(service.SendMessageCallCount()).To(Equal(1))
				sentMessage := service.SendMessageArgsForCall(0)

				Expect(*sentMessage.QueueUrl).To(Equal(expectedQueueUrl))
			})

			Context("when SQS returns an error", func() {
				BeforeEach(func() {
					service.SendMessageReturns(nil, errors.New("disaster"))
				})

				It("returns the error", func() {
					task := &queuefakes.FakeTask{}
					err := sqsQueue.Enqueue(task)
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Describe("retrieving work from the queue", func() {
			expectedHandle := "handle"

			BeforeEach(func() {
				output := &sqs.ReceiveMessageOutput{
					Messages: []*sqs.Message{
						{ReceiptHandle: &expectedHandle},
					},
				}

				service.ReceiveMessageReturns(output, nil)
			})

			It("retrieval is successful", func() {
				_, err := sqsQueue.Dequeue()
				Expect(err).ToNot(HaveOccurred())

				Expect(service.ReceiveMessageCallCount()).To(Equal(1))
				params := service.ReceiveMessageArgsForCall(0)
				Expect(params.QueueUrl).To(Equal(aws.String(expectedQueueUrl)))
				Expect(params.MaxNumberOfMessages).To(Equal(aws.Int64(1)))
				Expect(params.VisibilityTimeout).To(Equal(aws.Int64(60)))
				Expect(params.WaitTimeSeconds).To(Equal(aws.Int64(20)))
			})

			Context("receiving a message fails", func() {
				BeforeEach(func() {
					service.ReceiveMessageReturns(nil, errors.New("error receiving message"))
				})

				It("returns an error", func() {
					_, err := sqsQueue.Dequeue()
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Describe("removing work from the queue after we've done it", func() {
			expectedHandle := "handle"

			It("removal is successful", func() {
				task := &queuefakes.FakeTask{}
				task.ReceiptReturns(expectedHandle)

				err := sqsQueue.Remove(task)
				Expect(err).ToNot(HaveOccurred())

				Expect(service.DeleteMessageCallCount()).To(Equal(1))

				params := service.DeleteMessageArgsForCall(0)
				Expect(params.QueueUrl).To(Equal(aws.String(expectedQueueUrl)))
				Expect(params.ReceiptHandle).To(Equal(aws.String(expectedHandle)))
			})

			Context("removing a message fails", func() {
				BeforeEach(func() {
					service.DeleteMessageReturns(nil, errors.New("error receiving message"))
				})

				It("returns an error", func() {
					task := &queuefakes.FakeTask{}
					err := sqsQueue.Remove(task)
					Expect(err).To(HaveOccurred())
				})
			})
		})
	})

	Context("we cannot get the queue url", func() {
		BeforeEach(func() {
			service.GetQueueUrlReturns(nil, errors.New("disaster"))
		})

		Describe("creating the queue", func() {
			It("returns an error", func() {
				_, err := queue.BuildSQSQueue(service, "test")
				Expect(err).To(HaveOccurred())

				Expect(service.GetQueueUrlCallCount()).To(Equal(1))
				queueInput := service.GetQueueUrlArgsForCall(0)
				queueName := *queueInput.QueueName

				Expect(queueName).To(Equal(expectedQueueName))
			})
		})
	})

})

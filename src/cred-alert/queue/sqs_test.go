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
				task.IDReturns("task-id")
				task.TypeReturns("task-name")
				task.PayloadReturns(`{"arg-name": "arg-value"}`)

				err := sqsQueue.Enqueue(task)
				Expect(err).NotTo(HaveOccurred())

				Expect(service.SendMessageCallCount()).To(Equal(1))
				sentMessage := service.SendMessageArgsForCall(0)

				Expect(sentMessage.QueueUrl).To(Equal(aws.String(expectedQueueUrl)))
				Expect(*sentMessage.MessageBody).To(MatchJSON(`{"arg-name": "arg-value"}`))
				Expect(sentMessage.MessageAttributes).To(HaveKeyWithValue("type", &sqs.MessageAttributeValue{
					DataType:    aws.String("String"),
					StringValue: aws.String("task-name"),
				}))
				Expect(sentMessage.MessageAttributes).To(HaveKeyWithValue("id", &sqs.MessageAttributeValue{
					DataType:    aws.String("String"),
					StringValue: aws.String("task-id"),
				}))
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
			expectedMessageAttributes := map[string]*sqs.MessageAttributeValue{
				"id": {
					DataType:    aws.String("String"),
					StringValue: aws.String("task-id"),
				},
				"type": {
					DataType:    aws.String("String"),
					StringValue: aws.String("task-name"),
				},
			}
			messageBody := `{"arg-name": "arg-value"}`

			BeforeEach(func() {
				output := &sqs.ReceiveMessageOutput{
					Messages: []*sqs.Message{
						{
							ReceiptHandle:     aws.String(expectedHandle),
							MessageAttributes: expectedMessageAttributes,
							Body:              aws.String(messageBody),
						},
					},
				}

				service.ReceiveMessageReturns(output, nil)
			})

			It("retrieval is successful", func() {
				task, err := sqsQueue.Dequeue()
				Expect(err).ToNot(HaveOccurred())

				Expect(service.ReceiveMessageCallCount()).To(Equal(1))
				params := service.ReceiveMessageArgsForCall(0)
				Expect(params.QueueUrl).To(Equal(aws.String(expectedQueueUrl)))
				Expect(params.MaxNumberOfMessages).To(Equal(aws.Int64(1)))
				Expect(params.VisibilityTimeout).To(Equal(aws.Int64(60)))
				Expect(params.WaitTimeSeconds).To(Equal(aws.Int64(20)))
				Expect(params.MessageAttributeNames).To(Equal(aws.StringSlice([]string{"id", "type"})))

				Expect(task.ID()).To(Equal("task-id"))
				Expect(task.Type()).To(Equal("task-name"))
				Expect(task.Payload()).To(Equal(`{"arg-name": "arg-value"}`))
			})

			Context("if no messages are in the response", func() {
				BeforeEach(func() {
					firstCall := true

					service.ReceiveMessageStub = func(input *sqs.ReceiveMessageInput) (*sqs.ReceiveMessageOutput, error) {
						if firstCall {
							firstCall = false
							return &sqs.ReceiveMessageOutput{
								Messages: []*sqs.Message{},
							}, nil
						} else {
							return &sqs.ReceiveMessageOutput{
								Messages: []*sqs.Message{
									{
										ReceiptHandle:     aws.String(expectedHandle),
										MessageAttributes: expectedMessageAttributes,
										Body:              aws.String(messageBody),
									},
								},
							}, nil
						}
					}
				})
			})

			It("blocks until there is a message to process", func() {
				_, err := sqsQueue.Dequeue()
				Expect(err).ToNot(HaveOccurred())
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
			expectedMessageAttributes := map[string]*sqs.MessageAttributeValue{
				"id": {
					DataType:    aws.String("String"),
					StringValue: aws.String("task-id"),
				},
				"type": {
					DataType:    aws.String("String"),
					StringValue: aws.String("task-name"),
				},
			}
			messageBody := `{"arg-name": "arg-value"}`

			BeforeEach(func() {
				output := &sqs.ReceiveMessageOutput{
					Messages: []*sqs.Message{
						{
							ReceiptHandle:     &expectedHandle,
							MessageAttributes: expectedMessageAttributes,
							Body:              &messageBody,
						},
					},
				}

				service.ReceiveMessageReturns(output, nil)
			})

			It("removal is successful", func() {
				task, err := sqsQueue.Dequeue()
				Expect(err).ToNot(HaveOccurred())

				err = task.Ack()
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
					task, err := sqsQueue.Dequeue()
					Expect(err).ToNot(HaveOccurred())

					err = task.Ack()
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

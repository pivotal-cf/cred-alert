package queue

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
)

type sqsQueue struct {
	service  SQSAPI
	queueUrl *string
}

func BuildSQSQueue(service SQSAPI, queueName string) (*sqsQueue, error) {
	params := &sqs.GetQueueUrlInput{
		QueueName: aws.String(queueName),
	}
	resp, err := service.GetQueueUrl(params)
	if err != nil {
		return nil, err
	}

	url := resp.QueueUrl

	return &sqsQueue{
		service:  service,
		queueUrl: url,
	}, nil
}

func (q *sqsQueue) Enqueue(task Task) error {
	params := &sqs.SendMessageInput{
		MessageBody: aws.String("hello I am in the queue"), // TODO: how to encode?
		QueueUrl:    q.queueUrl,
	}

	if _, err := q.service.SendMessage(params); err != nil {
		return err
	}

	return nil
}

func (q *sqsQueue) Dequeue() (Task, error) {
	params := &sqs.ReceiveMessageInput{
		QueueUrl:            q.queueUrl,
		MaxNumberOfMessages: aws.Int64(1),
		VisibilityTimeout:   aws.Int64(60),
		WaitTimeSeconds:     aws.Int64(20),
	}

	if _, err := q.service.ReceiveMessage(params); err != nil {
		return nil, err
	}

	return nil, nil
}

func (q *sqsQueue) Remove(task Task) error {
	params := &sqs.DeleteMessageInput{
		QueueUrl:      q.queueUrl,
		ReceiptHandle: aws.String(task.Receipt()),
	}

	if _, err := q.service.DeleteMessage(params); err != nil {
		return err
	}

	return nil
}

//go:generate counterfeiter . SQSAPI

type SQSAPI interface {
	GetQueueUrl(*sqs.GetQueueUrlInput) (*sqs.GetQueueUrlOutput, error)

	SendMessage(*sqs.SendMessageInput) (*sqs.SendMessageOutput, error)

	ReceiveMessage(*sqs.ReceiveMessageInput) (*sqs.ReceiveMessageOutput, error)
	DeleteMessage(*sqs.DeleteMessageInput) (*sqs.DeleteMessageOutput, error)
}

package queue

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
)

type sqsQueue struct {
	service  SQSAPI
	queueUrl *string
}

const TaskIDAttributeName = "id"
const TaskTypeAttributeName = "type"

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
	args := map[string]*sqs.MessageAttributeValue{
		TaskIDAttributeName: &sqs.MessageAttributeValue{
			DataType:    aws.String("String"),
			StringValue: aws.String(task.ID()),
		},
		TaskTypeAttributeName: &sqs.MessageAttributeValue{
			DataType:    aws.String("String"),
			StringValue: aws.String(task.Type()),
		},
	}

	params := &sqs.SendMessageInput{
		MessageBody:       aws.String(task.Payload()),
		MessageAttributes: args,
		QueueUrl:          q.queueUrl,
	}

	if _, err := q.service.SendMessage(params); err != nil {
		return err
	}

	return nil
}

func (q *sqsQueue) Dequeue() (AckTask, error) {
	params := &sqs.ReceiveMessageInput{
		QueueUrl:              q.queueUrl,
		MaxNumberOfMessages:   aws.Int64(1),
		VisibilityTimeout:     aws.Int64(60),
		WaitTimeSeconds:       aws.Int64(20),
		MessageAttributeNames: aws.StringSlice([]string{TaskIDAttributeName, TaskTypeAttributeName}),
	}

	var message *sqs.Message

	for {
		response, err := q.service.ReceiveMessage(params)
		if err != nil {
			return nil, err
		}

		if len(response.Messages) == 0 {
			continue
		}

		message = response.Messages[0]
		break
	}

	receiptHandle := message.ReceiptHandle
	id := *message.MessageAttributes[TaskIDAttributeName].StringValue
	typee := *message.MessageAttributes[TaskTypeAttributeName].StringValue
	payload := *message.Body

	return &sqsTask{
		queueURL:      q.queueUrl,
		receiptHandle: receiptHandle,
		id:            id,
		typee:         typee,
		payload:       payload,
		service:       q.service,
	}, nil
}

type sqsTask struct {
	queueURL      *string
	receiptHandle *string

	id      string
	typee   string
	payload string
	service SQSAPI
}

func (t *sqsTask) ID() string {
	return t.id
}

func (t *sqsTask) Type() string {
	return t.typee
}

func (t *sqsTask) Payload() string {
	return t.payload
}

func (t *sqsTask) Ack() error {
	params := &sqs.DeleteMessageInput{
		QueueUrl:      t.queueURL,
		ReceiptHandle: t.receiptHandle,
	}

	if _, err := t.service.DeleteMessage(params); err != nil {
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

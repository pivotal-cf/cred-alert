package main

import (
	"cred-alert/queue"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/jessevdk/go-flags"
	"github.com/pivotal-golang/lager"
)

type Opts struct {
	AWS struct {
		AccessKey       string `long:"aws-access-key" description:"access key for aws SQS service" value-name:"ACCESS_KEY" required:"true"`
		SecretAccessKey string `long:"aws-secret-key" description:"secret access key for aws SQS service" value-name:"SECRET_KEY" required:"true"`
		Region          string `long:"aws-region" description:"aws region for SQS service" value-name:"REGION" required:"true"`
	} `group:"AWS Options"`

	FromQueueName string `long:"from-queue" description:"queue to recieve messages from" value-name:"NAME" required:"true"`
	ToQueueName   string `long:"to-queue" description:"queue to send messages to" value-name:"NAME" required:"true"`
}

func main() {
	var opts Opts

	_, err := flags.ParseArgs(&opts, os.Args)
	if err != nil {
		os.Exit(1)
	}

	logger := lager.NewLogger("message-reviver")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.INFO))

	creds := credentials.NewStaticCredentials(
		opts.AWS.AccessKey,
		opts.AWS.SecretAccessKey,
		"",
	)
	config := aws.NewConfig().WithCredentials(creds).WithRegion(opts.AWS.Region)
	service := sqs.New(session.New(config))

	fromQueue, err := queue.BuildSQSQueue(service, opts.FromQueueName)
	if err != nil {
		logger.Fatal("building-from-queue-failed", err)
	}

	toQueue, err := queue.BuildSQSQueue(service, opts.ToQueueName)
	if err != nil {
		logger.Fatal("building-to-queue-failed", err)
	}

	var task queue.AckTask

	do(logger, "dequeuing", func() error {
		var err error
		task, err = fromQueue.Dequeue()
		return err
	})

	do(logger, "enqueuing", func() error {
		return toQueue.Enqueue(task)
	})

	do(logger, "acking", func() error {
		return task.Ack()
	})
}

func do(logger lager.Logger, name string, work func() error) {
	l := logger.Session(name)

	l.Info("starting")

	err := work()
	if err != nil {
		l.Fatal("failed", err)
	}

	l.Info("done")
}

package queue

import (
	"cloud.google.com/go/pubsub"
	"code.cloudfoundry.org/lager"
	"golang.org/x/net/context"
)

//go:generate counterfeiter . Topic

type Topic interface {
	Publish(context.Context, ...*pubsub.Message) ([]string, error)
}

type pubSubEnqueuer struct {
	logger lager.Logger
	topic  Topic
}

func NewPubSubEnqueuer(logger lager.Logger, topic Topic) Enqueuer {
	return &pubSubEnqueuer{
		logger: logger,
		topic:  topic,
	}
}

func (p *pubSubEnqueuer) Enqueue(task Task) error {
	message := &pubsub.Message{
		Attributes: map[string]string{
			"id":   task.ID(),
			"type": task.Type(),
		},
		Data: []byte(task.Payload()),
	}

	_, err := p.topic.Publish(context.TODO(), message)
	if err != nil {
		p.logger.Error("failed-to-publish", err)
		return err
	}

	p.logger.Info("successfully-published", lager.Data{
		"id":   task.ID(),
		"type": task.Type(),
	})

	return nil
}

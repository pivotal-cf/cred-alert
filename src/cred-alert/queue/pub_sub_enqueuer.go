package queue

import (
	"cred-alert/crypto"
	"encoding/base64"

	"cloud.google.com/go/pubsub"
	"code.cloudfoundry.org/lager"
	"golang.org/x/net/context"
)

//go:generate counterfeiter . Topic

type Topic interface {
	Publish(context.Context, *pubsub.Message) *pubsub.PublishResult
}

type pubSubEnqueuer struct {
	logger lager.Logger
	topic  Topic
	signer crypto.Signer
}

func NewPubSubEnqueuer(logger lager.Logger, topic Topic, signer crypto.Signer) Enqueuer {
	return &pubSubEnqueuer{
		logger: logger,
		topic:  topic,
		signer: signer,
	}
}

func (p *pubSubEnqueuer) Enqueue(task Task) error {
	payload := []byte(task.Payload())
	signature, err := p.signer.Sign(payload)
	if err != nil {
		p.logger.Error("failed-to-sign", err)
		return err
	}

	endcodedSignature := base64.StdEncoding.EncodeToString(signature)

	message := &pubsub.Message{
		Attributes: map[string]string{
			"id":        task.ID(),
			"type":      task.Type(),
			"signature": endcodedSignature,
		},
		Data: payload,
	}

	ctx := context.TODO()
	res := p.topic.Publish(ctx, message)
	id, err := res.Get(ctx)
	if err != nil {
		p.logger.Error("failed-to-publish", err)
		return err
	}

	p.logger.Info("successfully-published", lager.Data{
		"id":        task.ID(),
		"pubsub-id": id,
		"type":      task.Type(),
	})

	return nil
}

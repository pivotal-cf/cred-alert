package queue

import (
	"cred-alert/db"

	"cloud.google.com/go/pubsub"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . RetryHandler

type RetryHandler interface {
	ProcessMessage(lager.Logger, *pubsub.Message)
}

type retryHandler struct {
	failedMessageRepo db.FailedMessageRepository
	processor         PubSubProcessor
	acker             Acker
}

func NewRetryHandler(failedMessageRepo db.FailedMessageRepository, processor PubSubProcessor, acker Acker) RetryHandler {
	return &retryHandler{
		failedMessageRepo: failedMessageRepo,
		processor:         processor,
		acker:             acker,
	}
}

func (r *retryHandler) ProcessMessage(logger lager.Logger, msg *pubsub.Message) {
	retryable, err := r.processor.Process(logger, msg)

	if err != nil {
		logger.Error("failed-to-process-msg", err)

		if retryable {
			logger.Info("queuing-msg-for-retry")

			numRetries, err := r.failedMessageRepo.RegisterFailedMessage(logger, msg.ID)
			if err != nil {
				logger.Error("failed-to-save-msg", err)
			}

			if numRetries < 3 {
				r.acker.Ack(msg, false)
				return
			}

			r.failedMessageRepo.MarkFailedMessageAsDead(logger, msg.ID)
		}
	} else {
		r.failedMessageRepo.RemoveFailedMessage(logger, msg.ID)
	}

	r.acker.Ack(msg, true)
}

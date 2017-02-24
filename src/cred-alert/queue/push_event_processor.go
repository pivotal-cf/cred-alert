package queue

import (
	"bytes"
	"cred-alert/crypto"
	"cred-alert/metrics"
	"cred-alert/revok"
	"encoding/base64"
	"encoding/json"
	"errors"

	"cloud.google.com/go/pubsub"
	"code.cloudfoundry.org/lager"
)

type pushEventProcessor struct {
	changeFetcher       revok.ChangeFetcher
	verifier            crypto.Verifier
	verifyFailedCounter metrics.Counter
}

func NewPushEventProcessor(
	changeFetcher revok.ChangeFetcher,
	verifier crypto.Verifier,
	emitter metrics.Emitter,
) *pushEventProcessor {
	return &pushEventProcessor{
		changeFetcher:       changeFetcher,
		verifier:            verifier,
		verifyFailedCounter: emitter.Counter("queue.push_event_processor.verify.failed"),
	}
}

func (h *pushEventProcessor) Process(logger lager.Logger, message *pubsub.Message) (bool, error) {
	logger = logger.Session("processing-push-event")

	decodedSignature, err := base64.StdEncoding.DecodeString(message.Attributes["signature"])
	if err != nil {
		logger.Error("signature-malformed", err, lager.Data{
			"signature": message.Attributes["signature"],
		})
		return false, err
	}

	err = h.verifier.Verify(message.Data, decodedSignature)
	if err != nil {
		logger.Error("signature-invalid", err, lager.Data{
			"signature": message.Attributes["signature"],
		})
		h.verifyFailedCounter.Inc(logger)
		return false, err
	}

	decoder := json.NewDecoder(bytes.NewBuffer(message.Data))

	var p PushEventPlan
	err = decoder.Decode(&p)
	if err != nil {
		logger.Error("payload-malformed", err)
		return false, err
	}

	if p.Owner == "" || p.Repository == "" {
		err := errors.New("invalid payload: missing owner or repository")
		logger.Error("payload-incomplete", err)
		return false, err
	}

	logger = logger.WithData(lager.Data{
		"repository": p.Repository,
		"owner":      p.Owner,
	})

	err = h.changeFetcher.Fetch(logger, p.Owner, p.Repository, true)
	if err != nil {
		return true, err
	}

	return false, nil
}

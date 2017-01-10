package queue

import (
	"bytes"
	"cred-alert/crypto"
	"cred-alert/db"
	"cred-alert/metrics"
	"cred-alert/revok"
	"encoding/base64"
	"encoding/json"
	"errors"

	"cloud.google.com/go/pubsub"
	"code.cloudfoundry.org/lager"
)

type pushEventProcessor struct {
	db                  db.RepositoryRepository
	changeFetcher       revok.ChangeFetcher
	verifier            crypto.Verifier
	verifyFailedCounter metrics.Counter
}

func NewPushEventProcessor(
	changeFetcher revok.ChangeFetcher,
	db db.RepositoryRepository,
	verifier crypto.Verifier,
	emitter metrics.Emitter,
) *pushEventProcessor {
	return &pushEventProcessor{
		db:                  db,
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

	if len(p.Owner) == 0 || len(p.Repository) == 0 {
		err := errors.New("invalid payload: missing owner or repository")
		logger.Error("payload-incomplete", err)
		return false, err
	}

	logger = logger.WithData(lager.Data{
		"repository": p.Repository,
		"owner":      p.Owner,
	})

	repo, err := h.db.Find(p.Owner, p.Repository)
	if err != nil {
		logger.Error("repository-lookup-failed", err)
		return false, err
	}

	err = h.changeFetcher.Fetch(logger, repo)
	if err != nil {
		return true, err
	}

	return false, nil
}

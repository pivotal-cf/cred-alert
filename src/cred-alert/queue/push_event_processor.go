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
	changeDiscoverer    revok.ChangeDiscoverer
	verifier            crypto.Verifier
	verifyFailedCounter metrics.Counter
}

func NewPushEventProcessor(
	changeDiscoverer revok.ChangeDiscoverer,
	db db.RepositoryRepository,
	verifier crypto.Verifier,
	emitter metrics.Emitter,
) *pushEventProcessor {
	return &pushEventProcessor{
		db:                  db,
		changeDiscoverer:    changeDiscoverer,
		verifier:            verifier,
		verifyFailedCounter: emitter.Counter("queue.push_event_processor.verify.failed"),
	}
}

func (h *pushEventProcessor) Process(logger lager.Logger, message *pubsub.Message) (bool, error) {
	decodedSignature, err := base64.StdEncoding.DecodeString(message.Attributes["signature"])
	if err != nil {
		return false, err
	}

	err = h.verifier.Verify(message.Data, decodedSignature)
	if err != nil {
		h.verifyFailedCounter.Inc(logger)
		return false, err
	}

	decoder := json.NewDecoder(bytes.NewBuffer(message.Data))

	var p PushEventPlan
	err = decoder.Decode(&p)
	if err != nil {
		return false, err
	}

	if len(p.Owner) == 0 || len(p.Repository) == 0 {
		return false, errors.New("invalid payload: missing owner or repository")
	}

	repo, err := h.db.Find(p.Owner, p.Repository)
	if err != nil {
		return false, err
	}

	err = h.changeDiscoverer.Fetch(logger, repo)
	if err != nil {
		return true, err
	}

	return false, nil
}

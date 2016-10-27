package queue

import (
	"bytes"
	"cred-alert/db"
	"cred-alert/revok"
	"encoding/json"
	"errors"

	"cloud.google.com/go/pubsub"

	"code.cloudfoundry.org/lager"
)

type pushEventProcessor struct {
	logger           lager.Logger
	db               db.RepositoryRepository
	changeDiscoverer revok.ChangeDiscoverer
}

func NewPushEventProcessor(
	logger lager.Logger,
	changeDiscoverer revok.ChangeDiscoverer,
	db db.RepositoryRepository,
) *pushEventProcessor {
	return &pushEventProcessor{
		logger:           logger,
		db:               db,
		changeDiscoverer: changeDiscoverer,
	}
}

func (h *pushEventProcessor) Process(message *pubsub.Message) (bool, error) {
	decoder := json.NewDecoder(bytes.NewBuffer(message.Data))

	var p PushEventPlan
	err := decoder.Decode(&p)
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

	err = h.changeDiscoverer.Fetch(h.logger, repo)
	if err != nil {
		return true, err
	}

	return false, nil
}

package revok

import (
	"bytes"
	"cred-alert/db"
	"cred-alert/queue"
	"encoding/json"
	"errors"

	"cloud.google.com/go/pubsub"

	"code.cloudfoundry.org/lager"
)

type pushEventProcessor struct {
	db               db.RepositoryRepository
	changeDiscoverer ChangeDiscoverer
}

func NewPushEventProcessor(changeDiscoverer ChangeDiscoverer, db db.RepositoryRepository) *pushEventProcessor {
	return &pushEventProcessor{
		db:               db,
		changeDiscoverer: changeDiscoverer,
	}
}

func (h *pushEventProcessor) Process(logger lager.Logger, message *pubsub.Message) (bool, error) {
	decoder := json.NewDecoder(bytes.NewBuffer(message.Data))

	var p queue.PushEventPlan
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

	err = h.changeDiscoverer.Fetch(logger, repo)
	if err != nil {
		return true, err
	}

	return false, nil
}

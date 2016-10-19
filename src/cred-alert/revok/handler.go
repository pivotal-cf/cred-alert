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

type handler struct {
	logger           lager.Logger
	db               db.RepositoryRepository
	changeDiscoverer ChangeDiscoverer
}

func NewHandler(logger lager.Logger, changeDiscoverer ChangeDiscoverer, db db.RepositoryRepository) *handler {
	return &handler{
		logger:           logger,
		db:               db,
		changeDiscoverer: changeDiscoverer,
	}
}

func (h *handler) ProcessMessage(message *pubsub.Message) (bool, error) {
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

	err = h.changeDiscoverer.Fetch(h.logger, repo)
	if err != nil {
		return true, err
	}

	return false, nil
}

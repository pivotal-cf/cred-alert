package revok

import (
	"cred-alert/db"
	"cred-alert/queue"
	"encoding/json"
	"net/http"

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

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()

	var p queue.PushEventPlan
	err := decoder.Decode(&p)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if len(p.Owner) == 0 || len(p.Repository) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	repo, err := h.db.Find(p.Owner, p.Repository)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	err = h.changeDiscoverer.Fetch(h.logger, repo)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

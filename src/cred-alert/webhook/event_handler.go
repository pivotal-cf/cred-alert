package webhook

import (
	"cred-alert/git"
	"cred-alert/logging"

	myGithub "cred-alert/github"

	"github.com/google/go-github/github"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . EventHandler

type EventHandler interface {
	HandleEvent(lager.Logger, github.PushEvent)
}

type eventHandler struct {
	githubClient myGithub.Client
	scan         func(lager.Logger, string) []git.Line

	requestCounter    logging.Counter
	credentialCounter logging.Counter
}

func NewEventHandler(githubClient myGithub.Client, scan func(lager.Logger, string) []git.Line, emitter logging.Emitter) *eventHandler {
	requestCounter := emitter.Counter("cred_alert.webhook_requests")
	credentialCounter := emitter.Counter("cred_alert.violations")

	handler := &eventHandler{
		githubClient: githubClient,
		scan:         scan,

		requestCounter:    requestCounter,
		credentialCounter: credentialCounter,
	}

	return handler
}

func (s *eventHandler) HandleEvent(logger lager.Logger, event github.PushEvent) {
	logger = logger.Session("handle-event")

	s.requestCounter.Inc(logger)

	diff, err := s.githubClient.CompareRefs(logger, *event.Repo.Owner.Name, *event.Repo.Name, *event.Before, *event.After)
	if err != nil {
		return
	}

	lines := s.scan(logger, diff)
	for _, line := range lines {
		logger.Info("found-credential", lager.Data{
			"path":        line.Path,
			"line-number": line.LineNumber,
		})
	}

	s.credentialCounter.IncN(logger, len(lines))
}

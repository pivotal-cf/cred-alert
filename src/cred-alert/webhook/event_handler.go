package webhook

import (
	"cred-alert/git"
	"cred-alert/logging"
	"regexp"

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
	whitelist    []*regexp.Regexp

	requestCounter    logging.Counter
	credentialCounter logging.Counter
}

func NewEventHandler(githubClient myGithub.Client, scan func(lager.Logger, string) []git.Line, emitter logging.Emitter, whitelist []string) *eventHandler {
	requestCounter := emitter.Counter("cred_alert.webhook_requests")
	credentialCounter := emitter.Counter("cred_alert.violations")

	patterns := make([]*regexp.Regexp, len(whitelist))
	for i, uncompiled := range whitelist {
		patterns[i] = regexp.MustCompile(uncompiled)
	}

	handler := &eventHandler{
		githubClient: githubClient,
		scan:         scan,
		whitelist:    patterns,

		requestCounter:    requestCounter,
		credentialCounter: credentialCounter,
	}

	return handler
}

func (s *eventHandler) HandleEvent(logger lager.Logger, event github.PushEvent) {
	logger = logger.Session("handle-event")

	if s.isWhitelisted(event) {
		logger.Info("ignored-repo", lager.Data{
			"repo": *event.Repo.Name,
		})
		return
	}

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

func (s *eventHandler) isWhitelisted(event github.PushEvent) bool {
	if event.Repo.Name == nil {
		return false
	}

	for _, pattern := range s.whitelist {
		if pattern.MatchString(*event.Repo.Name) {
			return true
		}
	}

	return false
}

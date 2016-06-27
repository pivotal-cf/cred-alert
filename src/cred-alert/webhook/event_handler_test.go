package webhook_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cred-alert/git"
	"cred-alert/github/githubfakes"
	"cred-alert/logging/loggingfakes"
	"cred-alert/webhook"

	"github.com/google/go-github/github"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("EventHandler", func() {
	var (
		eventHandler     webhook.EventHandler
		logger           *lagertest.TestLogger
		emitter          *loggingfakes.FakeEmitter
		fakeGithubClient *githubfakes.FakeClient
	)

	BeforeEach(func() {
		scan := func(logger lager.Logger, diff string) []git.Line {
			lines := []git.Line{}

			return append(lines, git.Line{
				Path:       "path",
				LineNumber: 1,
				Content:    "content",
			})
		}

		emitter = &loggingfakes.FakeEmitter{}
		logger = lagertest.NewTestLogger("event-handler")
		fakeGithubClient = new(githubfakes.FakeClient)
		eventHandler = webhook.NewEventHandler(fakeGithubClient, scan, emitter)
	})

	It("counts violations in a push event", func() {
		someString := "some-string"
		eventHandler.HandleEvent(logger, github.PushEvent{
			Repo: &github.PushEventRepository{
				FullName: &someString,
				Name:     &someString,
				Owner: &github.PushEventRepoOwner{
					Name: &someString,
				},
			},
			Before: &someString,
			After:  &someString,
		})

		Expect(emitter.CountViolationCallCount()).To(Equal(1))
	})

	Context("when we fail to fetch the diff", func() {
		var wasScanned bool

		BeforeEach(func() {
			scan := func(logger lager.Logger, diff string) []git.Line {
				wasScanned = true

				return nil
			}

			emitter = &loggingfakes.FakeEmitter{}

			fakeGithubClient.CompareRefsReturns("", errors.New("disaster"))
			eventHandler = webhook.NewEventHandler(fakeGithubClient, scan, emitter)
		})

		It("does not try to scan the diff", func() {
			someString := github.String("some-string")
			eventHandler.HandleEvent(logger, github.PushEvent{
				Repo: &github.PushEventRepository{
					Name:     someString,
					FullName: someString,
					Owner: &github.PushEventRepoOwner{
						Name: someString,
					},
				},
				After:  someString,
				Before: someString,
			})

			Expect(wasScanned).To(BeFalse())
			Expect(emitter.CountViolationCallCount()).To(Equal(0))
		})
	})
})

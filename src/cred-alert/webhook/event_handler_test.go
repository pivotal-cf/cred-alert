package webhook_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cred-alert/git"
	"cred-alert/github/githubfakes"
	"cred-alert/metrics"
	"cred-alert/metrics/metricsfakes"
	"cred-alert/webhook"

	"github.com/google/go-github/github"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("EventHandler", func() {
	var (
		eventHandler     webhook.EventHandler
		logger           *lagertest.TestLogger
		emitter          *metricsfakes.FakeEmitter
		fakeGithubClient *githubfakes.FakeClient

		scanFunc func(lager.Logger, string) []git.Line

		requestCounter    *metricsfakes.FakeCounter
		credentialCounter *metricsfakes.FakeCounter

		whitelist []string
	)

	BeforeEach(func() {
		scanFunc = func(logger lager.Logger, diff string) []git.Line {
			return []git.Line{}
		}

		emitter = &metricsfakes.FakeEmitter{}
		requestCounter = &metricsfakes.FakeCounter{}
		credentialCounter = &metricsfakes.FakeCounter{}

		emitter.CounterStub = func(name string) metrics.Counter {
			switch name {
			case "cred_alert.webhook_requests":
				return requestCounter
			case "cred_alert.violations":
				return credentialCounter
			default:
				panic("unexpected counter name! " + name)
			}
		}

		logger = lagertest.NewTestLogger("event-handler")
		fakeGithubClient = new(githubfakes.FakeClient)
	})

	JustBeforeEach(func() {
		eventHandler = webhook.NewEventHandler(fakeGithubClient, scanFunc, emitter, whitelist)
	})

	It("emits count when it is invoked", func() {
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

		Expect(requestCounter.IncCallCount()).To(Equal(1))
	})

	Context("It has a whitelist of ignored repos", func() {
		var scanCount int
		BeforeEach(func() {
			scanCount = 0
			scanFunc = func(logger lager.Logger, diff string) []git.Line {
				scanCount++
				return []git.Line{}
			}
			whitelist = []string{"some-credentials"}
		})

		It("ignores patterns in whitelist", func() {
			someString := "some-string"
			repoName := "some-credentials"

			pushEvent := github.PushEvent{
				Repo: &github.PushEventRepository{
					FullName: &someString,
					Name:     &repoName,
					Owner: &github.PushEventRepoOwner{
						Name: &someString,
					},
				},
				Before: &someString,
				After:  &someString,
			}

			eventHandler.HandleEvent(logger, pushEvent)
			Expect(scanCount).To(BeZero())
			Expect(len(logger.LogMessages())).To(Equal(1))
			Expect(logger.LogMessages()[0]).To(ContainSubstring("ignored-repo"))
			Expect(logger.Logs()[0].Data["repo"]).To(Equal("some-credentials"))
		})
	})

	Context("when a credential is found", func() {
		BeforeEach(func() {
			scanFunc = func(logger lager.Logger, diff string) []git.Line {
				lines := []git.Line{}

				return append(lines, git.Line{
					Path:       "path",
					LineNumber: 1,
					Content:    "content",
				})
			}
		})

		It("emits count of the credentials it has found", func() {
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

			Expect(credentialCounter.IncNCallCount()).To(Equal(1))
		})
	})

	Context("when we fail to fetch the diff", func() {
		var wasScanned bool

		BeforeEach(func() {
			wasScanned = false

			fakeGithubClient.CompareRefsReturns("", errors.New("disaster"))

			scanFunc = func(logger lager.Logger, diff string) []git.Line {
				wasScanned = true

				return nil
			}
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
			Expect(credentialCounter.IncNCallCount()).To(Equal(0))
		})
	})
})

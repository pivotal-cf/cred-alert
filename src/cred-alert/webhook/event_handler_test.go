package webhook_test

import (
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cred-alert/github/githubfakes"
	"cred-alert/metrics"
	"cred-alert/metrics/metricsfakes"
	"cred-alert/notifications/notificationsfakes"
	"cred-alert/sniff"
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
		notifier         *notificationsfakes.FakeNotifier
		fakeGithubClient *githubfakes.FakeClient

		repoName     string
		repoFullName string

		sniffFunc func(lager.Logger, sniff.Scanner) []sniff.Line

		requestCounter      *metricsfakes.FakeCounter
		credentialCounter   *metricsfakes.FakeCounter
		ignoredEventCounter *metricsfakes.FakeCounter

		whitelist []string
		event     github.PushEvent
	)

	BeforeEach(func() {
		repoName = "my-awesome-repo"
		repoFullName = fmt.Sprintf("rad-co/%s", repoName)

		sniffFunc = func(lager.Logger, sniff.Scanner) []sniff.Line {
			return []sniff.Line{}
		}

		emitter = &metricsfakes.FakeEmitter{}
		notifier = &notificationsfakes.FakeNotifier{}
		requestCounter = &metricsfakes.FakeCounter{}
		credentialCounter = &metricsfakes.FakeCounter{}
		ignoredEventCounter = &metricsfakes.FakeCounter{}

		emitter.CounterStub = func(name string) metrics.Counter {
			switch name {
			case "cred_alert.webhook_requests":
				return requestCounter
			case "cred_alert.violations":
				return credentialCounter
			case "cred_alert.ignored_events":
				return ignoredEventCounter
			default:
				panic("unexpected counter name! " + name)
			}
		}

		logger = lagertest.NewTestLogger("event-handler")
		fakeGithubClient = new(githubfakes.FakeClient)

		someString := "some-string"
		event = github.PushEvent{
			Repo: &github.PushEventRepository{
				FullName: &repoFullName,
				Name:     &someString,
				Owner: &github.PushEventRepoOwner{
					Name: &someString,
				},
			},
			Before: &someString,
			After:  &someString,
			Commits: []github.PushEventCommit{
				github.PushEventCommit{ID: &someString},
			},
		}
	})

	JustBeforeEach(func() {
		eventHandler = webhook.NewEventHandler(fakeGithubClient, sniffFunc, emitter, notifier, whitelist)
	})

	Context("when there are multiple commits in a single event", func() {
		var before string = "before"
		var id0, id1, id2 string = "a", "b", "c"

		BeforeEach(func() {
			commit0 := github.PushEventCommit{ID: &id0}
			commit1 := github.PushEventCommit{ID: &id1}
			commit2 := github.PushEventCommit{ID: &id2}
			commits := []github.PushEventCommit{commit0, commit1, commit2}

			event.Before = &before
			event.Commits = commits
		})

		It("compares each commit individually", func() {
			eventHandler.HandleEvent(logger, event)

			fakeGithubClient.CompareRefsReturns("", errors.New("disaster"))
			Expect(fakeGithubClient.CompareRefsCallCount()).To(Equal(3))
			_, _, _, sha0, sha1 := fakeGithubClient.CompareRefsArgsForCall(0)
			Expect(sha0).To(Equal(before))
			Expect(sha1).To(Equal(id0))
			_, _, _, sha0, sha1 = fakeGithubClient.CompareRefsArgsForCall(1)
			Expect(sha0).To(Equal(id0))
			Expect(sha1).To(Equal(id1))
			_, _, _, sha0, sha1 = fakeGithubClient.CompareRefsArgsForCall(2)
			Expect(sha0).To(Equal(id1))
			Expect(sha1).To(Equal(id2))
		})
	})

	It("emits count when it is invoked", func() {
		eventHandler.HandleEvent(logger, event)

		Expect(requestCounter.IncCallCount()).To(Equal(1))
	})

	Context("It has a whitelist of ignored repos", func() {
		var scanCount int

		BeforeEach(func() {
			repoName = "some-credentials"

			scanCount = 0
			sniffFunc = func(lager.Logger, sniff.Scanner) []sniff.Line {
				scanCount++
				return []sniff.Line{}
			}
			whitelist = []string{repoName}
			event.Repo.Name = &repoName
		})

		It("ignores patterns in whitelist", func() {
			eventHandler.HandleEvent(logger, event)

			Expect(scanCount).To(BeZero())
			Expect(len(logger.LogMessages())).To(Equal(1))
			Expect(logger.LogMessages()[0]).To(ContainSubstring("ignored-repo"))
			Expect(logger.Logs()[0].Data["repo"]).To(Equal(repoName))
		})

		It("emits a count of ignored push events", func() {
			eventHandler.HandleEvent(logger, event)
			Expect(ignoredEventCounter.IncCallCount()).To(Equal(1))
		})
	})

	Context("when a credential is found", func() {
		var filePath string
		var sha0 string = "sha0"

		BeforeEach(func() {
			filePath = "some/file/path"

			sniffFunc = func(lager.Logger, sniff.Scanner) []sniff.Line {
				return []sniff.Line{sniff.Line{
					Path:       filePath,
					LineNumber: 1,
					Content:    "content",
				}}
			}

			event.Commits[0].ID = &sha0
		})

		It("emits count of the credentials it has found", func() {
			eventHandler.HandleEvent(logger, event)

			Expect(credentialCounter.IncNCallCount()).To(Equal(1))
		})

		It("sends a notification", func() {
			eventHandler.HandleEvent(logger, event)

			Expect(notifier.SendNotificationCallCount()).To(Equal(1))
			Expect(notifier.SendNotificationArgsForCall(0)).To(ContainSubstring(repoFullName))
			Expect(notifier.SendNotificationArgsForCall(0)).To(ContainSubstring(sha0))
			Expect(notifier.SendNotificationArgsForCall(0)).To(ContainSubstring(filePath + ":1"))
		})
	})

	Context("when we fail to fetch the diff", func() {
		var wasScanned bool

		BeforeEach(func() {
			wasScanned = false

			fakeGithubClient.CompareRefsReturns("", errors.New("disaster"))

			sniffFunc = func(lager.Logger, sniff.Scanner) []sniff.Line {
				wasScanned = true

				return nil
			}
		})

		It("does not try to scan the diff", func() {
			eventHandler.HandleEvent(logger, event)

			Expect(wasScanned).To(BeFalse())
			Expect(credentialCounter.IncNCallCount()).To(Equal(0))
		})
	})
})

package webhook_test

import (
	"cred-alert/fakes"
	"cred-alert/git"
	"cred-alert/logging"
	. "cred-alert/webhook"

	"github.com/google/go-github/github"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PushEventScanner", func() {

	var scanner *PushEventScanner
	var fakeDatadogClient *fakes.FakeClient
	var fakeGithubClient *fakes.FakeGithubClient

	BeforeEach(func() {
		scan := func(diff string) []git.Line {
			lines := []git.Line{}

			return append(lines, git.Line{
				Path:       "path",
				LineNumber: 1,
				Content:    "content",
			})
		}

		fakeDatadogClient = new(fakes.FakeClient)
		fakeGithubClient = new(fakes.FakeGithubClient)
		emitter := logging.NewEmitter(fakeDatadogClient)
		scanner = NewPushEventScanner(fakeGithubClient, scan, emitter)
	})

	It("Counts violations in a push event", func() {
		someString := "some-string"
		scanner.ScanPushEvent(github.PushEvent{
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

		Expect(fakeDatadogClient.PublishSeriesCallCount()).To(Equal(1))
	})
})

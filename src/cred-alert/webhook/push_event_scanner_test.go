package webhook_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cred-alert/datadog/datadogfakes"
	"cred-alert/git"
	"cred-alert/logging"
	"cred-alert/webhook"

	"github.com/google/go-github/github"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("PushEventScanner", func() {
	var (
		scanner    *webhook.PushEventScanner
		logger     *lagertest.TestLogger
		fakeClient *datadogfakes.FakeClient
	)

	BeforeEach(func() {
		fetchDiff := func(event github.PushEvent) (string, error) {
			return "", nil
		}
		scan := func(diff string) []git.Line {
			lines := []git.Line{}

			return append(lines, git.Line{
				Path:       "path",
				LineNumber: 1,
				Content:    "content",
			})
		}

		fakeClient = new(datadogfakes.FakeClient)
		emitter := logging.NewEmitter(fakeClient)
		logger = lagertest.NewTestLogger("scanner")
		scanner = webhook.NewPushEventScanner(fetchDiff, scan, emitter)
	})

	It("counts violations in a push event", func() {
		someString := "some-string"
		scanner.ScanPushEvent(logger, github.PushEvent{
			Repo: &github.PushEventRepository{
				FullName: &someString,
			},
			After: &someString,
		})

		Expect(fakeClient.PublishSeriesCallCount()).To(Equal(1))
	})
})

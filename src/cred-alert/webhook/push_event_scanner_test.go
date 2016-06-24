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
	var fakeClient *fakes.FakeClient

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

		fakeClient = new(fakes.FakeClient)
		emitter := logging.NewEmitter(fakeClient)
		scanner = NewPushEventScanner(fetchDiff, scan, emitter)
	})

	It("Counts violations in a push event", func() {
		someString := "some-string"
		scanner.ScanPushEvent(github.PushEvent{
			Repo: &github.PushEventRepository{
				FullName: &someString,
			},
			After: &someString,
		})

		Expect(fakeClient.PublishSeriesCallCount()).To(Equal(1))
	})
})

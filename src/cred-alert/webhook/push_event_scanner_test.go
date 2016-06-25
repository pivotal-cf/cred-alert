package webhook_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cred-alert/git"
	"cred-alert/logging/loggingfakes"
	"cred-alert/webhook"

	"github.com/google/go-github/github"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("PushEventScanner", func() {
	var (
		scanner *webhook.PushEventScanner
		logger  *lagertest.TestLogger
		emitter *loggingfakes.FakeEmitter
	)

	BeforeEach(func() {
		fetchDiff := func(logger lager.Logger, event github.PushEvent) (string, error) {
			return "", nil
		}

		scan := func(logger lager.Logger, diff string) []git.Line {
			lines := []git.Line{}

			return append(lines, git.Line{
				Path:       "path",
				LineNumber: 1,
				Content:    "content",
			})
		}

		emitter = &loggingfakes.FakeEmitter{}
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

		Expect(emitter.CountViolationCallCount()).To(Equal(1))
	})

	Context("when we fail to fetch the diff", func() {
		var wasScanned bool

		BeforeEach(func() {
			scan := func(logger lager.Logger, diff string) []git.Line {
				wasScanned = true

				return nil
			}

			fetchDiff := func(logger lager.Logger, event github.PushEvent) (string, error) {
				return "", errors.New("disaster")
			}
			emitter = &loggingfakes.FakeEmitter{}

			scanner = webhook.NewPushEventScanner(fetchDiff, scan, emitter)
		})

		It("does not try to scan the diff", func() {
			someString := "some-string"
			scanner.ScanPushEvent(logger, github.PushEvent{
				Repo: &github.PushEventRepository{
					FullName: &someString,
				},
				After: &someString,
			})

			Expect(wasScanned).To(BeFalse())
			Expect(emitter.CountViolationCallCount()).To(Equal(0))
		})
	})
})

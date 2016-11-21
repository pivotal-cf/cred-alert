package revok_test

import (
	"errors"

	"cred-alert/db"
	"cred-alert/db/dbfakes"
	"cred-alert/metrics"
	"cred-alert/metrics/metricsfakes"
	"cred-alert/notifications"
	"cred-alert/notifications/notificationsfakes"
	"cred-alert/revok"
	"cred-alert/revok/revokfakes"
	"cred-alert/sniff"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	git "github.com/libgit2/git2go"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Rescanner", func() {
	var (
		logger               *lagertest.TestLogger
		scanRepository       *dbfakes.FakeScanRepository
		credentialRepository *dbfakes.FakeCredentialRepository
		scanner              *revokfakes.FakeScanner
		notifier             *notificationsfakes.FakeNotifier
		emitter              *metricsfakes.FakeEmitter

		successMetric *metricsfakes.FakeCounter
		failedMetric  *metricsfakes.FakeCounter

		runner  ifrit.Runner
		process ifrit.Process
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("repodiscoverer")

		scanRepository = &dbfakes.FakeScanRepository{}
		scanRepository.ScansNotYetRunWithVersionReturns([]db.PriorScan{
			{
				ID:         1,
				Branch:     "some-branch",
				StartSHA:   "some-start-sha",
				StopSHA:    "",
				Repository: "some-repository",
				Owner:      "some-owner",
			},
			{
				ID:         2,
				Branch:     "some-other-branch",
				StartSHA:   "some-other-start-sha",
				StopSHA:    "some-stop-sha",
				Repository: "some-other-repository",
				Owner:      "some-other-owner",
			},
		}, nil)

		credentialRepository = &dbfakes.FakeCredentialRepository{}
		credentialRepository.ForScanWithIDStub = func(int) ([]db.Credential, error) {
			if credentialRepository.ForScanWithIDCallCount() == 1 {
				return []db.Credential{
					{
						Owner:      "some-owner",
						Repository: "some-repo",
						SHA:        "some-sha",
						Path:       "some-path",
						LineNumber: 1,
						MatchStart: 2,
						MatchEnd:   3,
						Private:    true,
					},
				}, nil
			}

			return []db.Credential{
				{
					Owner:      "some-other-owner",
					Repository: "some-other-repo",
					SHA:        "some-other-sha",
					Path:       "some-other-path",
					LineNumber: 1,
					MatchStart: 2,
					MatchEnd:   3,
					Private:    true,
				},
			}, nil
		}

		scanner = &revokfakes.FakeScanner{}
		notifier = &notificationsfakes.FakeNotifier{}

		emitter = &metricsfakes.FakeEmitter{}
		successMetric = &metricsfakes.FakeCounter{}
		failedMetric = &metricsfakes.FakeCounter{}
		emitter.CounterStub = func(name string) metrics.Counter {
			switch name {
			case "revok.rescanner.success":
				return successMetric
			case "revok.rescanner.failed":
				return failedMetric
			}
			return &metricsfakes.FakeCounter{}
		}
	})

	AfterEach(func() {
		ginkgomon.Interrupt(process)
		<-process.Wait()
	})

	JustBeforeEach(func() {
		runner = revok.NewRescanner(
			logger,
			scanRepository,
			credentialRepository,
			scanner,
			notifier,
			emitter,
		)
		process = ginkgomon.Invoke(runner)
	})

	It("tries to get the scans not yet run with the current rules version from the DB", func() {
		Eventually(scanRepository.ScansNotYetRunWithVersionCallCount).Should(Equal(1))
		_, rulesVersion := scanRepository.ScansNotYetRunWithVersionArgsForCall(0)
		Expect(rulesVersion).To(Equal(sniff.RulesVersion))
	})

	It("repeats each prior scan for the previous rules version", func() {
		Eventually(scanner.ScanNoNotifyCallCount).Should(Equal(2))

		_, owner, repository, _, branch, startSHA, stopSHA := scanner.ScanNoNotifyArgsForCall(0)
		Expect(owner).To(Equal("some-owner"))
		Expect(repository).To(Equal("some-repository"))
		Expect(branch).To(Equal("some-branch"))
		Expect(startSHA).To(Equal("some-start-sha"))
		Expect(stopSHA).To(Equal(""))

		_, owner, repository, _, branch, startSHA, stopSHA = scanner.ScanNoNotifyArgsForCall(1)
		Expect(owner).To(Equal("some-other-owner"))
		Expect(repository).To(Equal("some-other-repository"))
		Expect(branch).To(Equal("some-other-branch"))
		Expect(startSHA).To(Equal("some-other-start-sha"))
		Expect(stopSHA).To(Equal("some-stop-sha"))
	})

	It("gets the credentials for each prior scan", func() {
		Eventually(credentialRepository.ForScanWithIDCallCount).Should(Equal(2))
		Expect(credentialRepository.ForScanWithIDArgsForCall(0)).To(Equal(1))
		Expect(credentialRepository.ForScanWithIDArgsForCall(1)).To(Equal(2))
	})

	It("increments the success metric", func() {
		Eventually(successMetric.IncCallCount).Should(Equal(2))
	})

	Context("when no prior scans for the previous rules version are found", func() {
		BeforeEach(func() {
			scanRepository.ScansNotYetRunWithVersionReturns(nil, nil)
		})

		It("does nothing", func() {
			Consistently(credentialRepository.ForScanWithIDCallCount).Should(BeZero())
			Consistently(scanner.ScanNoNotifyCallCount).Should(BeZero())
			Consistently(notifier.SendBatchNotificationCallCount).Should(BeZero())
		})
	})

	Context("when finding old credentials fails", func() {
		BeforeEach(func() {
			credentialRepository.ForScanWithIDStub = func(int) ([]db.Credential, error) {
				if credentialRepository.ForScanWithIDCallCount() == 1 {
					return nil, errors.New("an-error")
				}

				return []db.Credential{
					{
						Owner:      "some-other-owner",
						Repository: "some-other-repo",
						SHA:        "some-other-sha",
						Path:       "some-other-path",
						LineNumber: 1,
						MatchStart: 2,
						MatchEnd:   3,
						Private:    true,
					},
				}, nil
			}
		})

		It("increments the failed metric for the failed repository", func() {
			Eventually(failedMetric.IncCallCount).Should(Equal(1))
		})

		It("continues to the next prior scan for the previous rules version", func() {
			Eventually(credentialRepository.ForScanWithIDCallCount).Should(Equal(2))
			Eventually(scanner.ScanNoNotifyCallCount).Should(Equal(1))
			Consistently(scanner.ScanNoNotifyCallCount).Should(Equal(1))
		})
	})

	Context("when a credential is found that was not previously found", func() {
		BeforeEach(func() {
			credentialRepository.ForScanWithIDStub = func(int) ([]db.Credential, error) {
				if credentialRepository.ForScanWithIDCallCount() == 1 {
					return []db.Credential{
						{
							Owner:      "some-owner",
							Repository: "some-repo",
							SHA:        "some-sha",
							Path:       "some-path",
							LineNumber: 1,
							MatchStart: 2,
							MatchEnd:   3,
							Private:    true,
						},
					}, nil
				}

				return []db.Credential{
					{
						Owner:      "some-other-owner",
						Repository: "some-other-repo",
						SHA:        "some-other-sha",
						Path:       "some-other-path",
						LineNumber: 1,
						MatchStart: 2,
						MatchEnd:   3,
						Private:    true,
					},
				}, nil
			}

			scanner.ScanNoNotifyStub = func(lager.Logger, string, string, map[git.Oid]struct{}, string, string, string) ([]db.Credential, error) {
				if scanner.ScanNoNotifyCallCount() == 1 {
					return []db.Credential{
						{
							Owner:      "some-owner",
							Repository: "some-repo",
							SHA:        "some-sha",
							Path:       "some-path",
							LineNumber: 1,
							MatchStart: 2,
							MatchEnd:   3,
							Private:    true,
						},
						{ // new
							Owner:      "some-owner",
							Repository: "some-repo",
							SHA:        "some-sha",
							Path:       "some-other-path",
							LineNumber: 2,
							MatchStart: 3,
							MatchEnd:   4,
							Private:    true,
						},
					}, nil
				}

				return []db.Credential{
					{
						Owner:      "some-other-owner",
						Repository: "some-other-repo",
						SHA:        "some-other-sha",
						Path:       "some-other-path",
						LineNumber: 1,
						MatchStart: 2,
						MatchEnd:   3,
						Private:    true,
					},
				}, nil
			}
		})

		It("sends a notification for the new credentials", func() {
			Eventually(notifier.SendBatchNotificationCallCount).Should(Equal(1))
			_, batch := notifier.SendBatchNotificationArgsForCall(0)
			Expect(batch).To(Equal([]notifications.Notification{
				{
					Owner:      "some-owner",
					Repository: "some-repo",
					SHA:        "some-sha",
					Path:       "some-other-path",
					LineNumber: 2,
					Private:    true,
				},
			}))
		})
	})

	Context("when no new credentials are found", func() {
		BeforeEach(func() {
			scanner.ScanNoNotifyStub = func(lager.Logger, string, string, map[git.Oid]struct{}, string, string, string) ([]db.Credential, error) {
				if scanner.ScanNoNotifyCallCount() == 1 {
					return []db.Credential{
						{
							Owner:      "some-owner",
							Repository: "some-repo",
							SHA:        "some-sha",
							Path:       "some-path",
							LineNumber: 1,
							MatchStart: 2,
							MatchEnd:   3,
							Private:    true,
						},
					}, nil
				}

				return []db.Credential{
					{
						Owner:      "some-other-owner",
						Repository: "some-other-repo",
						SHA:        "some-other-sha",
						Path:       "some-other-path",
						LineNumber: 1,
						MatchStart: 2,
						MatchEnd:   3,
						Private:    true,
					},
				}, nil
			}
		})

		It("doesn't send any notifications", func() {
			Consistently(notifier.SendBatchNotificationCallCount).Should(BeZero())
		})
	})

	Context("when getting prior scans fails", func() {
		BeforeEach(func() {
			scanRepository.ScansNotYetRunWithVersionReturns(nil, errors.New("an-error"))
		})

		It("does not try to scan anything", func() {
			Eventually(scanner.ScanNoNotifyCallCount).Should(BeZero())
		})
	})

	Context("when doing a scan fails", func() {
		BeforeEach(func() {
			scanner.ScanNoNotifyStub = func(lager.Logger, string, string, map[git.Oid]struct{}, string, string, string) ([]db.Credential, error) {
				if scanner.ScanNoNotifyCallCount() == 1 {
					return nil, errors.New("an-error")
				}

				return []db.Credential{}, nil
			}
		})

		It("should continue on to the next repository", func() {
			Eventually(scanner.ScanNoNotifyCallCount).Should(Equal(2))
			_, owner, repository, _, branch, startSHA, stopSHA := scanner.ScanNoNotifyArgsForCall(1)
			Expect(owner).To(Equal("some-other-owner"))
			Expect(repository).To(Equal("some-other-repository"))
			Expect(branch).To(Equal("some-other-branch"))
			Expect(startSHA).To(Equal("some-other-start-sha"))
			Expect(stopSHA).To(Equal("some-stop-sha"))
		})

		It("increments the failed metric for the failed repository", func() {
			Eventually(failedMetric.IncCallCount).Should(Equal(1))
		})

		It("does not increment the success metric for the failed repository", func() {
			Eventually(successMetric.IncCallCount).Should(Equal(1))
		})
	})
})

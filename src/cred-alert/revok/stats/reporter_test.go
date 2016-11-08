package stats_test

import (
	"cred-alert/db/dbfakes"
	"cred-alert/metrics"
	"cred-alert/metrics/metricsfakes"
	"cred-alert/revok/stats"
	"errors"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

var _ = Describe("Reporter", func() {
	var (
		logger          *lagertest.TestLogger
		clock           *fakeclock.FakeClock
		interval        time.Duration
		statsRepository *dbfakes.FakeStatsRepository
		emitter         *metricsfakes.FakeEmitter

		repoGauge         *metricsfakes.FakeGauge
		disabledRepoGauge *metricsfakes.FakeGauge
		fetchGauge        *metricsfakes.FakeGauge
		credentialGauge   *metricsfakes.FakeGauge

		runner  ifrit.Runner
		process ifrit.Process
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("reporter")
		clock = fakeclock.NewFakeClock(time.Now())
		emitter = &metricsfakes.FakeEmitter{}
		repoGauge = &metricsfakes.FakeGauge{}
		disabledRepoGauge = &metricsfakes.FakeGauge{}
		fetchGauge = &metricsfakes.FakeGauge{}
		credentialGauge = &metricsfakes.FakeGauge{}
		emitter.GaugeStub = func(name string) metrics.Gauge {
			switch name {
			case "revok.reporter.repo_count":
				return repoGauge
			case "revok.reporter.disabled_repo_count":
				return disabledRepoGauge
			case "revok.reporter.fetch_count":
				return fetchGauge
			case "revok.reporter.credential_count":
				return credentialGauge
			default:
				panic("unexpected metric!")
			}
		}

		interval = 10 * time.Second

		statsRepository = &dbfakes.FakeStatsRepository{}
		statsRepository.RepositoryCountReturns(1, nil)
		statsRepository.FetchCountReturns(2, nil)
		statsRepository.CredentialCountReturns(3, nil)
		statsRepository.DisabledRepositoryCountReturns(4, nil)

		runner = stats.NewReporter(
			logger,
			clock,
			interval,
			statsRepository,
			emitter,
		)
		process = ginkgomon.Invoke(runner)
	})

	AfterEach(func() {
		ginkgomon.Interrupt(process)
		<-process.Wait()
	})

	It("doesn't emit anything when it first starts up", func() {
		Consistently(repoGauge.UpdateCallCount).Should(BeZero())
		Consistently(fetchGauge.UpdateCallCount).Should(BeZero())
		Consistently(credentialGauge.UpdateCallCount).Should(BeZero())
	})

	Context("after an interval has passed", func() {
		BeforeEach(func() {
			clock.WaitForWatcherAndIncrement(interval)
		})

		It("emits the repo count", func() {
			Eventually(repoGauge.UpdateCallCount).Should(Equal(1))

			_, updateValue, _ := repoGauge.UpdateArgsForCall(0)
			Expect(updateValue).To(BeNumerically("==", 1))
		})

		It("emits the fetch count", func() {
			Eventually(fetchGauge.UpdateCallCount).Should(Equal(1))

			_, updateValue, _ := fetchGauge.UpdateArgsForCall(0)
			Expect(updateValue).To(BeNumerically("==", 2))
		})

		It("emits the credential count", func() {
			Eventually(credentialGauge.UpdateCallCount).Should(Equal(1))

			_, updateValue, _ := credentialGauge.UpdateArgsForCall(0)
			Expect(updateValue).To(BeNumerically("==", 3))
		})

		It("emits the disabled repositories count", func() {
			Eventually(disabledRepoGauge.UpdateCallCount).Should(Equal(1))

			_, updateValue, _ := disabledRepoGauge.UpdateArgsForCall(0)
			Expect(updateValue).To(BeNumerically("==", 4))
		})
	})

	It("emits the counts after many intervals have passed", func() {
		for i := 1; i <= 3; i++ {
			clock.WaitForWatcherAndIncrement(interval)
			Eventually(repoGauge.UpdateCallCount).Should(Equal(i))
			Eventually(fetchGauge.UpdateCallCount).Should(Equal(i))
			Eventually(credentialGauge.UpdateCallCount).Should(Equal(i))
		}
	})

	Context("if fetching one of the stats fails", func() {
		BeforeEach(func() {
			statsRepository.RepositoryCountReturns(0, errors.New("disaster"))

			clock.WaitForWatcherAndIncrement(interval)
		})

		It("still emits the ones it can", func() {
			Eventually(fetchGauge.UpdateCallCount).Should(Equal(1))
			Eventually(credentialGauge.UpdateCallCount).Should(Equal(1))

			Consistently(repoGauge.UpdateCallCount).Should(BeZero())
		})
	})
})

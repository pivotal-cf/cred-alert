package revok_test

import (
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/tedsuo/ifrit"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cred-alert/revok"
	"os"
	"sync"
	"time"
)

var _ = Describe("Schedule Runner", func() {
	var (
		runner  *revok.ScheduleRunner
		process ifrit.Process

		logger *lagertest.TestLogger
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("schedule-runner")
	})

	JustBeforeEach(func() {
		runner = revok.NewScheduleRunner(logger)
		process = ifrit.Invoke(runner)
	})

	AfterEach(func() {
		process.Signal(os.Kill)
		<-process.Wait()
	})

	Describe("scheduling work", func() {
		It("runs the work on that schedule", func() {
			wg := &sync.WaitGroup{}
			wg.Add(2)

			runner.ScheduleWork("@every 1s", func() {
				wg.Done()
			})

			done := make(chan struct{})

			go func() {
				wg.Wait()
				close(done)
			}()

			Eventually(done, 3*time.Second).Should(BeClosed())
		})

		It("does not exit until all the work that is currently in progress has finished", func() {
			wg := &sync.WaitGroup{}
			wg.Add(1)

			started := make(chan struct{})

			runner.ScheduleWork("@every 1s", func() {
				close(started)
				wg.Wait()
			})

			<-started

			process.Signal(os.Kill)

			Consistently(process.Wait()).ShouldNot(Receive())

			wg.Done()

			Eventually(process.Wait()).Should(Receive())
		})
	})
})

package revok_test

import (
	"errors"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/robfig/cron"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cred-alert/db"
	"cred-alert/db/dbfakes"
	"cred-alert/revok"
	"cred-alert/revok/revokfakes"
)

var _ = Describe("Change Scheduler", func() {
	var (
		repositoryRepo *dbfakes.FakeRepositoryRepository
		scheduler      *revokfakes.FakeScheduler
		fetcher        *revokfakes.FakeChangeFetcher

		logger *lagertest.TestLogger

		changeScheduler *revok.ChangeScheduler
	)

	BeforeEach(func() {
		repositoryRepo = &dbfakes.FakeRepositoryRepository{}
		fetcher = &revokfakes.FakeChangeFetcher{}
		scheduler = &revokfakes.FakeScheduler{}
		logger = lagertest.NewTestLogger("scheduler")
	})

	JustBeforeEach(func() {
		changeScheduler = revok.NewChangeScheduler(logger, repositoryRepo, scheduler, fetcher)
	})

	Describe("scheduling a single repository", func() {
		It("schedules a fetch for each active repository", func() {
			repo := db.Repository{
				Name:  "repo-name",
				Owner: "repo-owner",
			}

			changeScheduler.ScheduleRepo(logger, repo)

			Expect(scheduler.ScheduleWorkCallCount()).Should(Equal(1))

			_, submittedWork := scheduler.ScheduleWorkArgsForCall(0)

			Expect(fetcher.FetchCallCount()).To(BeZero())

			submittedWork()

			Expect(fetcher.FetchCallCount()).To(Equal(1))
			_, passedRepo := fetcher.FetchArgsForCall(0)
			Expect(passedRepo).To(Equal(repo))
		})
	})

	Describe("scheduling all active repositories", func() {
		Context("when there are active repositories", func() {
			var (
				repo1 db.Repository
				repo2 db.Repository
			)

			BeforeEach(func() {
				repo1 = db.Repository{
					Name:  "repo-name",
					Owner: "repo-owner",
				}

				repo2 = db.Repository{
					Name:  "other-repo-name",
					Owner: "other-repo-owner",
				}

				repositoryRepo.ActiveReturns([]db.Repository{
					repo1,
					repo2,
				}, nil)
			})

			It("schedules a fetch for each active repository", func() {
				err := changeScheduler.ScheduleActiveRepos(logger)
				Expect(err).NotTo(HaveOccurred())

				Expect(scheduler.ScheduleWorkCallCount()).Should(Equal(2))

				_, firstJob := scheduler.ScheduleWorkArgsForCall(0)
				_, secondJob := scheduler.ScheduleWorkArgsForCall(1)

				Expect(fetcher.FetchCallCount()).To(BeZero())

				firstJob()

				Expect(fetcher.FetchCallCount()).To(Equal(1))
				_, passedRepo := fetcher.FetchArgsForCall(0)
				Expect(passedRepo).To(Equal(repo1))

				secondJob()

				Expect(fetcher.FetchCallCount()).To(Equal(2))
				_, passedRepo = fetcher.FetchArgsForCall(1)
				Expect(passedRepo).To(Equal(repo2))
			})

			It("distributes fetches across a time period", func() {
				err := changeScheduler.ScheduleActiveRepos(logger)
				Expect(err).NotTo(HaveOccurred())

				firstCron, _ := scheduler.ScheduleWorkArgsForCall(0)
				secondCron, _ := scheduler.ScheduleWorkArgsForCall(1)

				_, err = cron.Parse(firstCron)
				Expect(err).NotTo(HaveOccurred())

				_, err = cron.Parse(secondCron)
				Expect(err).NotTo(HaveOccurred())

				Expect(firstCron).ToNot(Equal(secondCron))
			})
		})

		Context("when we fail to fetch the active repositories", func() {
			BeforeEach(func() {
				repositoryRepo.ActiveReturns(nil, errors.New("disaster"))
			})

			It("returns an error", func() {
				err := changeScheduler.ScheduleActiveRepos(logger)
				Expect(err).To(HaveOccurred())
			})
		})
	})
})

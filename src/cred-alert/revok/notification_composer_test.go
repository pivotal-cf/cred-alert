package revok_test

import (
	"errors"

	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cred-alert/db"
	"cred-alert/db/dbfakes"
	"cred-alert/notifications"
	"cred-alert/notifications/notificationsfakes"
	"cred-alert/revok"
	"cred-alert/revok/revokfakes"
)

var _ = Describe("NotificationComposer", func() {
	var (
		notificationComposer revok.NotificationComposer

		logger               *lagertest.TestLogger
		repositoryRepository *dbfakes.FakeRepositoryRepository
		scanner              *revokfakes.FakeScanner
		router               *notificationsfakes.FakeRouter

		scannedShas map[string]struct{}
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("revok-scanner")
		repositoryRepository = &dbfakes.FakeRepositoryRepository{}
		scanner = &revokfakes.FakeScanner{}
		repositoryRepository.MustFindReturns(db.Repository{
			Model: db.Model{
				ID: 42,
			},
			Owner:   "some-owner",
			Name:    "some-repository",
			Private: true,
		}, nil)

		sha := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"

		scannedShas = map[string]struct{}{
			sha: {},
		}

		router = &notificationsfakes.FakeRouter{}

		notificationComposer = revok.NewNotificationComposer(
			repositoryRepository,
			router,
			scanner,
		)
	})

	var scanAndNotify = func() error {
		return notificationComposer.ScanAndNotify(
			logger,
			"some-owner",
			"some-repo",
			scannedShas,
			"some-branch",
			"start-sha",
			"stop-sha",
		)
	}

	Describe("ScanAndNotify", func() {
		It("finds the repository", func() {
			scanAndNotify()

			Expect(repositoryRepository.MustFindCallCount()).To(Equal(1))

			owner, repo := repositoryRepository.MustFindArgsForCall(0)
			Expect(owner).To(Equal("some-owner"))
			Expect(repo).To(Equal("some-repo"))
		})

		It("scans the repository", func() {
			scanAndNotify()

			Expect(scanner.ScanCallCount()).To(Equal(1))

			passedLogger, owner, repository, passedShas, branch, startSHA, stopSHA := scanner.ScanArgsForCall(0)
			Expect(passedLogger).To(Equal(logger))
			Expect(owner).To(Equal("some-owner"))
			Expect(repository).To(Equal("some-repo"))
			Expect(passedShas).To(Equal(scannedShas))
			Expect(branch).To(Equal("some-branch"))
			Expect(startSHA).To(Equal("start-sha"))
			Expect(stopSHA).To(Equal("stop-sha"))
		})

		Context("when credentials are found", func() {
			BeforeEach(func() {
				credentials := []db.Credential{
					{
						Owner:      "some-owner",
						Repository: "some-repo",
						SHA:        "some-sha",
						Path:       "some-path",
						LineNumber: 2,
					},
				}

				scanner.ScanReturns(credentials, nil)
			})

			It("does not return an error", func() {
				err := scanAndNotify()
				Expect(err).NotTo(HaveOccurred())
			})

			It("asks the router to deliver messages", func() {
				scanAndNotify()

				Expect(router.DeliverCallCount()).To(Equal(1))

				expectedBatch := []notifications.Notification{
					{
						Owner:      "some-owner",
						Repository: "some-repo",
						SHA:        "some-sha",
						Path:       "some-path",
						LineNumber: 2,
						Branch:     "some-branch",
						Private:    true,
					},
				}

				passedLogger, batch := router.DeliverArgsForCall(0)
				Expect(passedLogger).To(Equal(logger))
				Expect(batch).To(Equal(expectedBatch))
			})

			Context("when the router fails", func() {
				var expectedError error

				BeforeEach(func() {
					expectedError = errors.New("errrrror")
					router.DeliverReturns(expectedError)
				})

				It("returns an error", func() {
					err := scanAndNotify()
					Expect(err).To(Equal(expectedError))
				})
			})
		})

		Context("when no credentials are found", func() {
			BeforeEach(func() {
				credentials := []db.Credential{}
				scanner.ScanReturns(credentials, nil)
			})

			It("does not return an error", func() {
				err := scanAndNotify()
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not ask the router to deliver messages", func() {
				scanAndNotify()
				Expect(router.DeliverCallCount()).To(BeZero())
			})
		})

		Context("if repository repository errors", func() {
			BeforeEach(func() {
				repositoryRepository.MustFindReturns(db.Repository{}, errors.New("an-error"))
			})

			It("returns an error", func() {
				err := scanAndNotify()
				Expect(err).To(HaveOccurred())
			})

			It("does not scan", func() {
				scanAndNotify()
				Expect(scanner.ScanCallCount()).To(BeZero())
			})
		})

		Context("when the scanner fails", func() {
			BeforeEach(func() {
				scanner.ScanReturns([]db.Credential{}, errors.New("special-error"))
			})

			It("returns an error", func() {
				err := scanAndNotify()
				Expect(err).To(HaveOccurred())
			})

			It("does not try to deliver messages", func() {
				scanAndNotify()
				Expect(router.DeliverCallCount()).To(BeZero())
			})
		})
	})
})

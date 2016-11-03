package revok_test

import (
	"cred-alert/db"
	"cred-alert/db/dbfakes"
	"cred-alert/revok"
	"errors"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Fetch Interval Updater", func() {
	var (
		fetchIntervalUpdater revok.FetchIntervalUpdater
		repositoryRepository *dbfakes.FakeRepositoryRepository
		repo                 *db.Repository
		minimumInterval      time.Duration
		maximumInterval      time.Duration
	)

	BeforeEach(func() {
		repositoryRepository = &dbfakes.FakeRepositoryRepository{}

		minimumInterval = 6 * time.Hour
		maximumInterval = 168 * time.Hour

		fetchIntervalUpdater = revok.NewFetchIntervalUpdater(
			repositoryRepository,
			minimumInterval,
			maximumInterval,
		)

		repo = &db.Repository{}
	})

	var ItSetsTheCorrectInterval = func(
		lastActivity time.Duration,
		err error,
		expectedInterval time.Duration,
	) {
		repositoryRepository.LastActivityReturns(time.Now().Add(lastActivity), err)

		fetchIntervalUpdater.UpdateFetchInterval(repo)

		Expect(repositoryRepository.LastActivityCallCount()).To(Equal(1))
		passedRepo := repositoryRepository.LastActivityArgsForCall(0)
		Expect(passedRepo).To(BeIdenticalTo(repo))

		Expect(repositoryRepository.UpdateFetchIntervalCallCount()).To(Equal(1))
		passedRepo, fetchInterval := repositoryRepository.UpdateFetchIntervalArgsForCall(0)
		Expect(passedRepo).To(BeIdenticalTo(repo))
		Expect(fetchInterval).To(BeNumerically("~", expectedInterval, 1*time.Hour))
	}

	It("sets the correct fetch interval if last activity was less than a day", func() {
		ItSetsTheCorrectInterval(-23*time.Hour, nil, minimumInterval)
	})

	It("sets the correct fetch interval if last activity was between a day and a week", func() {
		ItSetsTheCorrectInterval(-24*time.Hour, nil, 18*time.Hour)
	})

	It("sets the correct fetch interval if last activity was between a day and a week", func() {
		ItSetsTheCorrectInterval(-167*time.Hour, nil, 18*time.Hour)
	})

	It("sets the correct fetch interval if last activity was between a week and two weeks", func() {
		ItSetsTheCorrectInterval(-168*time.Hour, nil, 54*time.Hour)
	})

	It("sets the correct fetch interval if last activity was between a week and two weeks", func() {
		ItSetsTheCorrectInterval(-335*time.Hour, nil, 54*time.Hour)
	})

	It("sets the correct fetch interval if last activity was over two weeks ago", func() {
		ItSetsTheCorrectInterval(-336*time.Hour, nil, maximumInterval)
	})

	It("sets the correct fetch interval if the repository's never been fetched", func() {
		ItSetsTheCorrectInterval(0, db.NeverBeenFetchedError, minimumInterval)
	})

	It("sets the correct fetch interval if the repository's never been changed", func() {
		ItSetsTheCorrectInterval(0, db.NoChangesError, maximumInterval)
	})

	It("returns an error if it fails to update the fetch interval", func() {
		expectedError := errors.New("My Special Error")

		repositoryRepository.UpdateFetchIntervalReturns(expectedError)

		err := fetchIntervalUpdater.UpdateFetchInterval(repo)
		Expect(err).To(Equal(expectedError))
	})
})

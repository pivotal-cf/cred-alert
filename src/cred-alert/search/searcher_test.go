package search_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cred-alert/db"
	"cred-alert/db/dbfakes"
	"cred-alert/gitclient"
	"cred-alert/gitclient/gitclientfakes"
	"cred-alert/search"
	"cred-alert/sniff/matchers"
)

var _ = Describe("Searcher", func() {
	var (
		repoRepository *dbfakes.FakeRepositoryRepository
		looper         *gitclientfakes.FakeLooper

		searcher search.Searcher
	)

	BeforeEach(func() {
		repoRepository = &dbfakes.FakeRepositoryRepository{}
		looper = &gitclientfakes.FakeLooper{}

		searcher = search.NewSearcher(repoRepository, looper)
	})

	Context("when there are repositories", func() {
		BeforeEach(func() {
			repos := []db.Repository{
				{
					Owner: "some-owner",
					Name:  "some-name",
					Path:  "some-repo-path",
				},
				{
					Owner: "some-other-owner",
					Name:  "some-other-name",
					Path:  "some-other-repo-path",
				},
			}

			repoRepository.ActiveReturns(repos, nil)

			looper.ScanCurrentStateStub = func(path string, callback gitclient.ScanCallback) error {
				switch path {
				case "some-repo-path":
					callback("abc123", "awesome-path/file.txt", []byte("goodbye\nyo hello, adele\nfrom the other side\n"))
				case "some-other-repo-path":
					callback("def456", "awesome-path/other-file.txt", []byte("hi hi hi\nhello, you!"))
				default:
					panic("called with an unexpected repository path!: " + path)
				}

				return nil
			}
		})

		It("scans the blobs in each repository with the matcher", func() {
			matcher := matchers.Format("hello, (.*)")
			results := searcher.SearchCurrent(matcher)

			result := search.Result{}

			Eventually(results.C()).Should(Receive(&result))
			Expect(result.Owner).To(Equal("some-owner"))
			Expect(result.Repository).To(Equal("some-name"))
			Expect(result.Revision).To(Equal("abc123"))
			Expect(result.Path).To(Equal("awesome-path/file.txt"))
			Expect(result.LineNumber).To(Equal(2))
			Expect(result.Location).To(Equal(3))
			Expect(result.Length).To(Equal(12))
			Expect(result.Content).To(Equal([]byte("yo hello, adele")))

			Eventually(results.C()).Should(Receive(&result))
			Expect(result.Owner).To(Equal("some-other-owner"))
			Expect(result.Repository).To(Equal("some-other-name"))
			Expect(result.Revision).To(Equal("def456"))
			Expect(result.Path).To(Equal("awesome-path/other-file.txt"))
			Expect(result.LineNumber).To(Equal(2))
			Expect(result.Location).To(Equal(0))
			Expect(result.Length).To(Equal(11))
			Expect(result.Content).To(Equal([]byte("hello, you!")))

			Consistently(results.C()).ShouldNot(Receive())
			Eventually(results.C()).Should(BeClosed())

			Expect(results.Err()).NotTo(HaveOccurred())
		})
	})

	Context("when we fail to get the repositories", func() {
		BeforeEach(func() {
			repoRepository.ActiveReturns(nil, errors.New("disaster"))
		})

		It("closes the channel with an error", func() {
			matcher := matchers.Format("gonna fail anyway")
			results := searcher.SearchCurrent(matcher)

			Eventually(results.C()).Should(BeClosed())
			Expect(results.Err()).To(HaveOccurred())
		})
	})
})

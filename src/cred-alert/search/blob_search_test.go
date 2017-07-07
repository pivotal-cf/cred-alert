package search_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/lager/lagertest"

	"cred-alert/db"
	"cred-alert/db/dbfakes"
	"cred-alert/gitclient/gitclientfakes"
	"cred-alert/search"
	"errors"
)

var _ = Describe("BlobSearch", func() {
	var (
		repoRepository *dbfakes.FakeRepositoryRepository
		fileLookup     *gitclientfakes.FakeFileLookup
		logger         *lagertest.TestLogger

		searcher search.BlobSearcher
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("searcher")

		repoRepository = &dbfakes.FakeRepositoryRepository{}
		fileLookup = &gitclientfakes.FakeFileLookup{}

		searcher = search.NewBlobSearcher(repoRepository, fileLookup)
	})

	Describe("ListBlobs", func() {
		Context("repository is known", func() {
			BeforeEach(func() {
				repoRepository.MustFindReturns(db.Repository{
					Path:          "/path/to/repository",
					DefaultBranch: "branch-name",
				}, nil)
			})

			Context("blobs.yml exists and is valid yaml", func() {
				BeforeEach(func() {
					fileLookup.FileContentsReturns([]byte(`
git/git-2.10.1.tar.gz:
  size: 6057915
  object_id: 55d1a453-2c54-4b96-80dd-1674575711b7
  sha: 8da73bdfc3d351ba8806ea85d8e29df18b47252d
golang/golang-linux-amd64.tar.gz:
  size: 90029041
  object_id: 8bbe0e29-7baf-4034-518a-ea98505a504d
  sha: 838c415896ef5ecd395dfabde5e7e6f8ac943c8e
  `), nil)
				})

				It("returns a list of blobs", func() {
					blobs, err := searcher.ListBlobs(logger, "owner-name", "repo-name")
					Expect(err).NotTo(HaveOccurred())
					Expect(blobs).To(ConsistOf([]search.BlobResult{
						{
							Path: "git/git-2.10.1.tar.gz",
							SHA:  "8da73bdfc3d351ba8806ea85d8e29df18b47252d",
						},
						{
							Path: "golang/golang-linux-amd64.tar.gz",
							SHA:  "838c415896ef5ecd395dfabde5e7e6f8ac943c8e",
						},
					}))

					ownerName, repoName := repoRepository.MustFindArgsForCall(0)
					Expect(ownerName).To(Equal("owner-name"))
					Expect(repoName).To(Equal("repo-name"))

					lookupPath, lookupVersion, lookupFile := fileLookup.FileContentsArgsForCall(0)
					Expect(lookupPath).To(Equal("/path/to/repository"))
					Expect(lookupVersion).To(Equal("branch-name"))
					Expect(lookupFile).To(Equal("config/blobs.yml"))
				})
			})

			Context("blobs.yml is invalid", func() {
				BeforeEach(func() {
					fileLookup.FileContentsReturns([]byte("invalid"), nil)
				})

				It("errors", func() {
					_, err := searcher.ListBlobs(logger, "owner-name", "repo-name")
					Expect(err).To(HaveOccurred())
				})
			})

			Context("config/blobs.yml does not exist", func() {
				BeforeEach(func() {
					fileLookup.FileContentsReturns(nil, errors.New("no such file"))
				})

				It("errors", func() {
					_, err := searcher.ListBlobs(logger, "owner-name", "repo-name")
					Expect(err).To(MatchError("no such file"))
				})
			})

		})

		Context("repository does not exist", func() {
			BeforeEach(func() {
				repoRepository.MustFindReturns(db.Repository{}, errors.New("repo not found"))
			})

			It("errors", func() {
				_, err := searcher.ListBlobs(logger, "owner-name", "repo-name")
				Expect(err).To(MatchError("repo not found"))
			})
		})
	})
})

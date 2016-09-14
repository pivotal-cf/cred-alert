package revok_test

import (
	"cred-alert/db/dbfakes"
	"cred-alert/gitclient/gitclientfakes"
	"cred-alert/revok"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Cloner", func() {
	var (
		runner               ifrit.Runner
		process              ifrit.Process
		workdir              string
		workCh               chan revok.CloneMsg
		logger               *lagertest.TestLogger
		gitClient            *gitclientfakes.FakeClient
		repositoryRepository *dbfakes.FakeRepositoryRepository
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("repodiscoverer")
		workCh = make(chan revok.CloneMsg, 10)
		gitClient = &gitclientfakes.FakeClient{}
		repositoryRepository = &dbfakes.FakeRepositoryRepository{}

		var err error
		workdir, err = ioutil.TempDir("", "revok-test")
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		runner = revok.NewCloner(
			logger,
			workdir,
			workCh,
			gitClient,
			repositoryRepository,
		)
		process = ginkgomon.Invoke(runner)
	})

	AfterEach(func() {
		ginkgomon.Interrupt(process)
		os.RemoveAll(workdir)
	})

	Context("when there is a message on the clone message channel", func() {
		BeforeEach(func() {
			workCh <- revok.CloneMsg{
				URL:        "some-url",
				Repository: "some-repo",
				Owner:      "some-owner",
			}
		})

		It("tries to clone when it receives a message", func() {
			Eventually(gitClient.CloneCallCount).Should(Equal(1))
			url, dest := gitClient.CloneArgsForCall(0)
			Expect(url).To(Equal("some-url"))
			Expect(dest).To(Equal(filepath.Join(workdir, "some-owner", "some-repo")))
		})

		It("updates the repository in the database", func() {
			Eventually(repositoryRepository.MarkAsClonedCallCount).Should(Equal(1))
		})

		Context("when cloning fails", func() {
			BeforeEach(func() {
				gitClient.CloneStub = func(url, dest string) error {
					err := os.MkdirAll(dest, os.ModePerm)
					Expect(err).NotTo(HaveOccurred())
					return errors.New("an-error")
				}
			})

			It("cleans up the failed clone destination, if any", func() {
				Eventually(gitClient.CloneCallCount).Should(Equal(1))
				_, dest := gitClient.CloneArgsForCall(0)
				Eventually(dest).ShouldNot(BeADirectory())
			})
		})
	})
})

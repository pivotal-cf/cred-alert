package revok_test

import (
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cred-alert/revok"
	"cred-alert/revok/revokfakes"

	"github.com/google/go-github/github"
)

var _ = Describe("GitHubClient", func() {
	var (
		ghService  *revokfakes.FakeGithubRepositoryService
		testLogger lager.Logger

		ghClient *revok.GitHubClient
	)

	BeforeEach(func() {
		ghService = &revokfakes.FakeGithubRepositoryService{}
		testLogger = lagertest.NewTestLogger("GitHubClient")

		someName := "some-name"
		someUserName := "some-user"
		someUser := github.User{Login: &someUserName}
		someURL := "some-url"
		someBool := true
		someBranch := "some-branch"

		repo := &github.Repository{
			Name:          &someName,
			Owner:         &someUser,
			SSHURL:        &someURL,
			Private:       &someBool,
			DefaultBranch: &someBranch,
		}
		ghService.GetReturns(repo, nil, nil)
		ghClient = revok.NewGitHubClient(ghService)

	})

	Describe("GetRepo", func() {
		It("gets a repo and exits successfully", func() {
			owner := "some-owner"
			repoName := "some-repo"

			_, err := ghClient.GetRepo(testLogger, owner, repoName)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

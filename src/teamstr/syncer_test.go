package teamstr_test

import (
	"context"

	"github.com/google/go-github/github"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"


	"teamstr"
	"teamstr/teamstrfakes"

)

var _ = Describe("Syncer", func() {
	var (
		orgRepo  *teamstrfakes.FakeOrgRepo
		repoRepo *teamstrfakes.FakeRepoRepo

		syncer *teamstr.Syncer
	)

	BeforeEach(func() {
		orgRepo = &teamstrfakes.FakeOrgRepo{}
		repoRepo = &teamstrfakes.FakeRepoRepo{}

		syncer = teamstr.NewSyncer(orgRepo, repoRepo)
	})

	It("adds all organization repos to a team", func() {
		orgRepoCalls := 0
		orgRepo.ListTeamReposStub = func(context context.Context, team int, opt *github.ListOptions) ([]*github.Repository, *github.Response, error) {
			orgRepoCalls++
			switch orgRepoCalls {
			case 1:
				return []*github.Repository{
						{Name: github.String("repo-a")},
						{Name: github.String("repo-b")},
						{Name: github.String("repo-c")},
					}, &github.Response{
						NextPage: 2,
					}, nil
			case 2:
				return []*github.Repository{
						{Name: github.String("repo-d")},
						{Name: github.String("repo-e")},
						{Name: github.String("repo-f")},
					}, &github.Response{
						NextPage: 0,
					}, nil
			default:
				panic("called too many times!")
			}
		}

		repoRepoCalls := 0
		repoRepo.ListByOrgStub = func(context context.Context, org string, opt *github.RepositoryListByOrgOptions) ([]*github.Repository, *github.Response, error) {
			repoRepoCalls++
			switch repoRepoCalls {
			case 1:
				return []*github.Repository{
						{Name: github.String("repo-a")},
						{Name: github.String("repo-b")},
						{Name: github.String("repo-c")},
					}, &github.Response{
						NextPage: 2,
					}, nil
			case 2:
				return []*github.Repository{
						{Name: github.String("repo-d")},
						{Name: github.String("repo-e")},
						{Name: github.String("repo-f")},
					}, &github.Response{
						NextPage: 3,
					}, nil
			case 3:
				return []*github.Repository{
						{Name: github.String("repo-g")},
						{Name: github.String("repo-h")},
					}, &github.Response{
						NextPage: 0,
					}, nil
			default:
				panic("called too many times!")
			}
		}

		err := syncer.Swim("example-org", 123456)
		Expect(err).NotTo(HaveOccurred())

		Expect(orgRepo.AddTeamRepoCallCount()).To(Equal(2))

		_, addedTeam, addedOwner, addedRepo, addedOpts := orgRepo.AddTeamRepoArgsForCall(0)
		Expect(addedTeam).To(Equal(123456))
		Expect(addedOwner).To(Equal("example-org"))
		Expect(addedRepo).To(Equal("repo-g"))
		Expect(addedOpts).To(Equal(&github.OrganizationAddTeamRepoOptions{
			Permission: "pull",
		}))

		_, addedTeam, addedOwner, addedRepo, addedOpts = orgRepo.AddTeamRepoArgsForCall(1)
		Expect(addedTeam).To(Equal(123456))
		Expect(addedOwner).To(Equal("example-org"))
		Expect(addedRepo).To(Equal("repo-h"))
		Expect(addedOpts).To(Equal(&github.OrganizationAddTeamRepoOptions{
			Permission: "pull",
		}))
	})
})

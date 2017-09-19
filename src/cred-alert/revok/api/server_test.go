package api_test

import (
	"context"
	"errors"

	"code.cloudfoundry.org/lager/lagertest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"cred-alert/db"
	"cred-alert/db/dbfakes"
	"cred-alert/revok/api"
	"cred-alert/revokpb"
)

var _ = Describe("Server", func() {
	var (
		repositoryRepository *dbfakes.FakeRepositoryRepository
		branchRepository     *dbfakes.FakeBranchRepository
		s                    *api.Server
	)

	BeforeEach(func() {
		logger := lagertest.NewTestLogger("revok-server")

		branchRepository = &dbfakes.FakeBranchRepository{}
		repositoryRepository = &dbfakes.FakeRepositoryRepository{}

		s = api.NewServer(logger, repositoryRepository, branchRepository)
	})

	Describe("GetCredentialCounts", func() {
		var (
			response *revokpb.CredentialCountResponse
			err      error
		)

		BeforeEach(func() {
			branchRepository.GetCredentialCountByOwnerReturns([]db.OwnerCredentialCount{
				{
					Owner:           "some-other-owner",
					CredentialCount: 11,
				},
				{
					Owner:           "some-owner",
					CredentialCount: 10,
				},
			}, nil)

			request := &revokpb.CredentialCountRequest{}
			response, err = s.GetCredentialCounts(context.Background(), request)
		})

		It("does not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns counts for all repositories in owner-alphabetical order", func() {
			occ1 := &revokpb.OrganizationCredentialCount{
				Owner: "some-other-owner",
				Count: 11,
			}

			occ2 := &revokpb.OrganizationCredentialCount{
				Owner: "some-owner",
				Count: 10,
			}

			Expect(response).NotTo(BeNil())
			Expect(response.CredentialCounts).NotTo(BeNil())
			Expect(response.CredentialCounts).To(Equal([]*revokpb.OrganizationCredentialCount{occ1, occ2}))
		})

		Context("when the database returns an error", func() {
			BeforeEach(func() {
				branchRepository.GetCredentialCountByOwnerReturns(nil, errors.New("disaster"))
			})

			It("errors", func() {
				request := &revokpb.CredentialCountRequest{}
				_, err = s.GetCredentialCounts(context.Background(), request)
				Expect(err).To(HaveOccurred())
			})

		})
	})

	Describe("GetOrganizationCredentialCounts", func() {
		var (
			response *revokpb.OrganizationCredentialCountResponse
			err      error
		)

		BeforeEach(func() {
			branchRepository.GetCredentialCountForOwnerReturns([]db.RepositoryCredentialCount{
				{
					Owner:           "some-owner",
					Name:            "repo-name-1",
					CredentialCount: 14,
					Private:         true,
				},
				{
					Owner:           "some-owner",
					Name:            "repo-name-2",
					CredentialCount: 2,
					Private:         false,
				},
			}, nil)

			request := &revokpb.OrganizationCredentialCountRequest{}
			response, err = s.GetOrganizationCredentialCounts(context.Background(), request)
		})

		It("does not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns counts and privacy value for all repositories in owner-alphabetical order", func() {
			occ1 := &revokpb.RepositoryCredentialCount{
				Owner:   "some-owner",
				Name:    "repo-name-1",
				Private: true,
				Count:   14,
			}

			occ2 := &revokpb.RepositoryCredentialCount{
				Owner:   "some-owner",
				Name:    "repo-name-2",
				Private: false,
				Count:   2,
			}

			Expect(response).NotTo(BeNil())
			Expect(response.CredentialCounts).NotTo(BeNil())
			Expect(response.CredentialCounts).To(Equal([]*revokpb.RepositoryCredentialCount{occ1, occ2}))
		})

		Context("when the database returns an error", func() {
			BeforeEach(func() {
				branchRepository.GetCredentialCountForOwnerReturns(nil, errors.New("disaster"))
			})

			It("errors", func() {
				request := &revokpb.OrganizationCredentialCountRequest{}
				_, err = s.GetOrganizationCredentialCounts(context.Background(), request)
				Expect(err).To(HaveOccurred())
			})

		})
	})

	Describe("GetRepositoryCredentialCounts", func() {
		BeforeEach(func() {
			branchRepository.GetCredentialCountForRepoReturns([]db.BranchCredentialCount{
				{
					Owner:           "some-owner",
					Name:            "repo-name",
					Branch:          "branch-1",
					CredentialCount: 14,
				},
				{
					Owner:           "some-owner",
					Name:            "repo-name",
					Branch:          "branch-2",
					CredentialCount: 2,
				},
			}, nil)

			repositoryRepository.FindReturns(
				db.Repository{
					Name:    "some-repo",
					Private: true,
				},
				true,
				nil,
			)

		})

		It("returns counts for all branches in alphabetical order", func() {
			occ1 := &revokpb.BranchCredentialCount{
				Name:  "branch-1",
				Count: 14,
			}

			occ2 := &revokpb.BranchCredentialCount{
				Name:  "branch-2",
				Count: 2,
			}

			request := &revokpb.RepositoryCredentialCountRequest{}
			response, err := s.GetRepositoryCredentialCounts(context.Background(), request)
			Expect(err).NotTo(HaveOccurred())

			Expect(response).NotTo(BeNil())
			Expect(response.CredentialCounts).NotTo(BeNil())
			Expect(response.CredentialCounts).To(Equal([]*revokpb.BranchCredentialCount{occ1, occ2}))
		})

		Context("when looking up credential counts returns an error", func() {
			BeforeEach(func() {
				branchRepository.GetCredentialCountForRepoReturns(
					nil,
					errors.New("credential count disaster"),
				)
			})

			It("errors", func() {
				request := &revokpb.RepositoryCredentialCountRequest{}
				_, err := s.GetRepositoryCredentialCounts(context.Background(), request)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when looking up repository returns an error", func() {
			BeforeEach(func() {
				repositoryRepository.FindReturns(db.Repository{}, false, errors.New("repository disaster"))
			})

			It("errors", func() {
				request := &revokpb.RepositoryCredentialCountRequest{}
				_, err := s.GetRepositoryCredentialCounts(context.Background(), request)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when repository is not found", func() {
			BeforeEach(func() {
				repositoryRepository.FindReturns(db.Repository{}, false, nil)
			})

			It("errors", func() {
				request := &revokpb.RepositoryCredentialCountRequest{}
				_, err := s.GetRepositoryCredentialCounts(context.Background(), request)
				Expect(err).To(HaveOccurred())
				Expect(grpc.Code(err)).To(Equal(codes.NotFound))
			})
		})
	})
})

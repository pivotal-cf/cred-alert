package revok_test

import (
	"context"
	"cred-alert/db"
	"cred-alert/db/dbfakes"
	"cred-alert/revok"
	"cred-alert/revokpb"

	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RevokServer", func() {
	Describe("GetCredentialCounts", func() {
		var (
			repoDB *dbfakes.FakeRepositoryRepository
			server revok.RevokServer

			ctx     context.Context
			request *revokpb.CredentialCountRequest

			response *revokpb.CredentialCountResponse
			err      error
		)

		BeforeEach(func() {
			logger := lagertest.NewTestLogger("revok-server")
			repoDB = &dbfakes.FakeRepositoryRepository{}
			repoDB.AllReturns([]db.Repository{
				{
					Owner: "some-owner",
					CredentialCounts: db.PropertyMap{
						"o1r1b1": float64(1),
						"o1r1b2": float64(2),
					},
				},
				{
					Owner: "some-owner",
					CredentialCounts: db.PropertyMap{
						"o1r2b1": float64(3),
						"o1r2b2": float64(4),
					},
				},
				{
					Owner: "some-other-owner",
					CredentialCounts: db.PropertyMap{
						"o2b1": float64(5),
						"o2b2": float64(6),
					},
				},
			}, nil)

			ctx = context.Background()
			request = &revokpb.CredentialCountRequest{}

			server = revok.NewRevokServer(logger, repoDB)
		})

		JustBeforeEach(func() {
			response, err = server.GetCredentialCounts(ctx, request)
		})

		It("gets repositories from the database", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(repoDB.AllCallCount()).To(Equal(1))
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
	})
})

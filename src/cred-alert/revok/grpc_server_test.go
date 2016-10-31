package revok_test

import (
	"context"
	"cred-alert/revok"
	"cred-alert/revok/revokfakes"
	"cred-alert/revokpb"
	"fmt"
	"os"
	"time"

	"google.golang.org/grpc"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("GrpcServer", func() {
	var (
		logger      lager.Logger
		revokServer *revokfakes.FakeRevokServer

		listenAddr string
		runner     ifrit.Runner
		process    ifrit.Process
	)

	BeforeEach(func() {
		listenAddr = fmt.Sprintf(":%d", GinkgoParallelNode()+9000)
		revokServer = &revokfakes.FakeRevokServer{}
		revokServer.GetCredentialCountsReturns(&revokpb.CredentialCountResponse{
			CredentialCounts: []*revokpb.CredentialCount{
				{
					Owner:      "some-owner",
					Repository: "some-repo",
					Count:      42,
				},
			},
		}, nil)
	})

	JustBeforeEach(func() {
		logger = lagertest.NewTestLogger("grpc-server")

		runner = revok.NewGRPCServer(logger, listenAddr, revokServer)
		process = ginkgomon.Invoke(runner)
	})

	AfterEach(func() {
		ginkgomon.Interrupt(process)
	})

	It("exits when signaled", func() {
		process.Signal(os.Interrupt)
		Eventually(process.Wait()).Should(Receive())
	})

	Context("when given a request", func() {
		var (
			conn    *grpc.ClientConn
			client  revokpb.RevokClient
			request *revokpb.CredentialCountRequest
		)

		BeforeEach(func() {
			var err error
			conn, err = grpc.Dial(listenAddr, grpc.WithInsecure())
			Expect(err).NotTo(HaveOccurred())
			client = revokpb.NewRevokClient(conn)
			request = &revokpb.CredentialCountRequest{
				Organizations: []*revokpb.Organization{
					{
						Name: "some-org",
						Repositories: []*revokpb.Repository{
							{
								Name: "some-repo",
							},
						},
					},
				},
			}
		})

		AfterEach(func() {
			conn.Close()
		})

		It("hits the revok server for credentials", func() {
			// there is a race between the server listening and us marking it as
			// being ready. this tries to hit it repeatedly to wait for it to
			// actually be listening.
			Eventually(func() error {
				_, err := client.GetCredentialCounts(context.Background(), request)
				return err
			}, 2*time.Second).ShouldNot(HaveOccurred())
			Eventually(revokServer.GetCredentialCountsCallCount).Should(Equal(1))
		})
	})
})

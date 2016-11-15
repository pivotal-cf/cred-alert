package revok_test

import (
	"context"
	"cred-alert/revok"
	"cred-alert/revok/revokfakes"
	"cred-alert/revokpb"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var _ = Describe("GrpcServer", func() {
	var (
		logger lager.Logger
		server *revokfakes.FakeServer

		listenAddr   string
		runner       ifrit.Runner
		process      ifrit.Process
		rootCertPool *x509.CertPool
	)

	BeforeEach(func() {
		listenAddr = fmt.Sprintf(":%d", GinkgoParallelNode()+9000)
		server = &revokfakes.FakeServer{}
		server.GetCredentialCountsReturns(&revokpb.CredentialCountResponse{
			CredentialCounts: []*revokpb.OrganizationCredentialCount{
				{
					Owner: "some-owner",
					Count: 42,
				},
			},
		}, nil)

		rootCertPool = x509.NewCertPool()
		bs, err := ioutil.ReadFile(filepath.Join("fixtures", "rootCA.crt"))
		Expect(err).NotTo(HaveOccurred())
		success := rootCertPool.AppendCertsFromPEM(bs)
		Expect(success).To(BeTrue())
	})

	JustBeforeEach(func() {
		logger = lagertest.NewTestLogger("grpc-server")

		certificate, err := tls.LoadX509KeyPair(
			filepath.Join("fixtures", "server.crt"),
			filepath.Join("fixtures", "server.key"),
		)
		Expect(err).NotTo(HaveOccurred())

		config := &tls.Config{
			ClientAuth:   tls.RequireAndVerifyClientCert,
			Certificates: []tls.Certificate{certificate},
			ClientCAs:    rootCertPool,
		}
		runner = revok.NewGRPCServer(logger, listenAddr, server, config)
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
			clientCert, err := tls.LoadX509KeyPair(
				filepath.Join("fixtures", "client.crt"),
				filepath.Join("fixtures", "client.key"),
			)

			clientTransportCreds := credentials.NewTLS(&tls.Config{
				ServerName:   "127.0.0.1",
				Certificates: []tls.Certificate{clientCert},
				RootCAs:      rootCertPool,
			})

			dialOption := grpc.WithTransportCredentials(clientTransportCreds)
			conn, err = grpc.Dial(listenAddr, dialOption)
			Expect(err).NotTo(HaveOccurred())

			client = revokpb.NewRevokClient(conn)

			request = &revokpb.CredentialCountRequest{}
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
			Eventually(server.GetCredentialCountsCallCount).Should(Equal(1))
		})
	})
})

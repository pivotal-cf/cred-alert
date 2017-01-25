package redrunner_test

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"red/redpb"
	"red/redrunner"
)

type DummyServer struct {
	callCount int
}

func (d *DummyServer) CallCount() int {
	return d.callCount
}

func (d *DummyServer) DummyCall(ctx context.Context, e *empty.Empty) (*empty.Empty, error) {
	d.callCount++

	return e, nil
}

var _ = Describe("GRPC Server", func() {
	var (
		logger      lager.Logger
		dummyServer *DummyServer

		listenAddr   string
		runner       ifrit.Runner
		process      ifrit.Process
		rootCertPool *x509.CertPool
	)

	BeforeEach(func() {
		listenAddr = fmt.Sprintf(":%d", GinkgoParallelNode()+9000)
		dummyServer = &DummyServer{}

		rootCertPool = x509.NewCertPool()
		bs, err := ioutil.ReadFile(filepath.Join("fixtures", "rootCA.crt"))
		Expect(err).NotTo(HaveOccurred())
		success := rootCertPool.AppendCertsFromPEM(bs)
		Expect(success).To(BeTrue())

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
		runner = redrunner.NewGRPCServer(logger, listenAddr, config, func(server *grpc.Server) {
			redpb.RegisterDummyServer(server, dummyServer)
		})
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
			conn   *grpc.ClientConn
			client redpb.DummyClient
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

			conn, err = grpc.Dial(
				listenAddr,
				grpc.WithTransportCredentials(clientTransportCreds),
				grpc.WithBlock(),
			)
			Expect(err).NotTo(HaveOccurred())

			client = redpb.NewDummyClient(conn)
		})

		AfterEach(func() {
			conn.Close()
		})

		It("hits the revok server for credentials", func() {
			_, err := client.DummyCall(context.Background(), &empty.Empty{})
			Expect(err).NotTo(HaveOccurred())

			Expect(dummyServer.CallCount()).To(Equal(1))
		})
	})
})

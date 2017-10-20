package main

import (
	"crypto/tls"
	"fmt"
	"html/template"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"code.cloudfoundry.org/lager"
	"github.com/robdimsdale/honeylager"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
	"github.com/tedsuo/rata"

	"cred-alert/ccp/api"
	"cred-alert/ccp/web"
	"cred-alert/config"
	"cred-alert/revokpb"
)

const (
	// Required.
	portEnvKey = "PORT"

	// Passphrase for the client private key. Required if the key is encrypted.
	clientKeyPassphraseEnvKey = "CLIENT_KEY_PASSPHRASE"

	// Address for RPC server. Required.
	rpcServerAddressEnvKey = "RPC_SERVER_ADDRESS"

	// Port for RPC server. Required.
	rpcServerPortEnvKey = "RPC_SERVER_PORT"

	// Required.
	caCertEnvKey = "SERVER_CA_CERT"

	// Required.
	clientCertEnvKey = "CLIENT_CERT"

	// Required.
	clientKeyEnvKey = "CLIENT_KEY"

	// Optional
	honeycombWriteKeyEnvKey = "HONEYCOMB_WRITE_KEY"
	honeycombDatasetEnvKey  = "HONEYCOMB_DATASET"
)

var (
	indexLayout        *template.Template
	organizationLayout *template.Template
	repositoryLayout   *template.Template
	logger             lager.Logger
)

func init() {
	bs, err := web.Asset("web/templates/index.html")
	if err != nil {
		log.Fatalf("failed loading asset: %s", err.Error())
	}
	indexLayout = template.Must(template.New("index.html").Parse(string(bs)))

	bs, err = web.Asset("web/templates/organization.html")
	if err != nil {
		log.Fatalf("failed loading asset: %s", err.Error())
	}
	organizationLayout = template.Must(template.New("organization.html").Parse(string(bs)))

	bs, err = web.Asset("web/templates/repository.html")
	if err != nil {
		log.Fatalf("failed loading asset: %s", err.Error())
	}
	repositoryLayout = template.Must(template.New("repository.html").Parse(string(bs)))

	logger = lager.NewLogger("credential-count-publisher")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.INFO))
}

func main() {
	honeycombWriteKey := os.Getenv(honeycombWriteKeyEnvKey)
	honeycombDataset := os.Getenv(honeycombDatasetEnvKey)
	if honeycombWriteKey != "" && honeycombDataset != "" {
		s := honeylager.NewSink(honeycombWriteKey, honeycombDataset, lager.DEBUG)
		defer s.Close()
		logger.RegisterSink(s)
	} else {
		logger.Info(fmt.Sprintf(
			"Honeycomb not configured - need %s and %s",
			honeycombWriteKeyEnvKey,
			honeycombDatasetEnvKey,
		))
	}

	logger.Info("starting")

	portStr := mustGetEnv(portEnvKey)
	port, err := strconv.Atoi(portStr)
	if err != nil {
		logger.Fatal("failed-to-parse-port", err)
	}

	rpcServerAddress := mustGetEnv(rpcServerAddressEnvKey)

	rpcServerPortStr := mustGetEnv(rpcServerPortEnvKey)
	rpcServerPort, err := strconv.Atoi(rpcServerPortStr)
	if err != nil {
		logger.Fatal("failed-to-parse-rpc-server-port", err)
	}

	clientCertStr := mustGetEnv(clientCertEnvKey)
	clientKeyStr := mustGetEnv(clientKeyEnvKey)

	serverAddr := fmt.Sprintf("%s:%d", rpcServerAddress, rpcServerPort)
	listenAddr := fmt.Sprintf(":%d", port)

	clientCert, err := config.LoadCertificate(
		[]byte(clientCertStr),
		[]byte(clientKeyStr),
		os.Getenv(clientKeyPassphraseEnvKey),
	)
	if err != nil {
		log.Fatalln(err)
	}

	caCert := mustGetEnv(caCertEnvKey)

	rootCertPool, err := config.LoadCertificatePool([]byte(caCert))
	if err != nil {
		log.Fatalln(err)
	}

	transportCreds := credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      rootCertPool,
	})

	conn, err := grpc.Dial(
		serverAddr,
		grpc.WithTransportCredentials(transportCreds),
		grpc.WithDialer(keepAliveDial),
		grpc.WithBlock(),
	)
	if err != nil {
		log.Fatalf("failed to create handler: %s", err.Error())
	}
	defer conn.Close()

	revokClient := revokpb.NewRevokAPIClient(conn)

	handler, err := rata.NewRouter(web.Routes, rata.Handlers{
		web.Index:        api.NewIndexHandler(logger, indexLayout, revokClient),
		web.Organization: api.NewOrganizationHandler(logger, organizationLayout, revokClient),
		web.Repository:   api.NewRepositoryHandler(logger, repositoryLayout, revokClient),
	})

	if err != nil {
		log.Fatalf("failed to create handler: %s", err.Error())
	}

	runner := sigmon.New(http_server.New(listenAddr, handler))

	err = <-ifrit.Invoke(runner).Wait()
	if err != nil {
		logger.Error("running-server-failed", err)
	}
}

func keepAliveDial(addr string, timeout time.Duration) (net.Conn, error) {
	d := net.Dialer{
		Timeout:   timeout,
		KeepAlive: 60 * time.Second,
	}
	return d.Dial("tcp", addr)
}

func mustGetEnv(key string) string {
	val := os.Getenv(key)
	err := fmt.Errorf("failed-to-get-env-key")
	if val == "" {
		logger.Fatal("failed-to-get-env-key", err, lager.Data{"missing-env-key": key})
	}

	return val
}

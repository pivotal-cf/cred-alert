package main

import (
	"crypto/tls"
	"fmt"
	"html/template"
	"log"
	"net"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"code.cloudfoundry.org/lager"
	flags "github.com/jessevdk/go-flags"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
	"github.com/tedsuo/rata"

	"cred-alert/config"
	"cred-alert/revok/api"
	"cred-alert/revok/web"
	"cred-alert/revokpb"
)

type Opts struct {
	Port uint16 `long:"port" default:"8080" description:"Port to listen on."`

	RPCServerAddress string `long:"rpc-server-address" description:"Address for RPC server." required:"true"`
	RPCServerPort    uint16 `long:"rpc-server-port" description:"Port for RPC server." required:"true"`

	CACertPath          string `long:"ca-cert-path" description:"Path to the CA certificate" required:"true"`
	ClientCertPath      string `long:"client-cert-path" description:"Path to the client certificate" required:"true"`
	ClientKeyPath       string `long:"client-key-path" description:"Path to the client private key" required:"true"`
	ClientKeyPassphrase string `long:"client-key-passphrase" description:"Passphrase for the client private key, if encrypted"`
}

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
	var opts Opts

	logger.Info("starting")

	_, err := flags.ParseArgs(&opts, os.Args)
	if err != nil {
		os.Exit(1)
	}

	serverAddr := fmt.Sprintf("%s:%d", opts.RPCServerAddress, opts.RPCServerPort)
	listenAddr := fmt.Sprintf(":%d", opts.Port)

	clientCert, err := config.LoadCertificate(
		opts.ClientCertPath,
		opts.ClientKeyPath,
		opts.ClientKeyPassphrase,
	)
	if err != nil {
		log.Fatalln(err)
	}

	rootCertPool, err := config.LoadCertificatePool(opts.CACertPath)
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
	)
	if err != nil {
		log.Fatalf("failed to create handler: %s", err.Error())
	}
	defer conn.Close()

	revokClient := revokpb.NewRevokClient(conn)

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

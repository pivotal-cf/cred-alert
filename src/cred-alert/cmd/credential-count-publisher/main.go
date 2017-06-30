package main

import (
	"crypto/tls"
	"fmt"
	"html/template"
	"log"
	"net"
	"os"
	"time"

	"golang.org/x/oauth2"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry-community/go-cfenv"
	"github.com/gorilla/context"
	"github.com/gorilla/sessions"
	flags "github.com/jessevdk/go-flags"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
	"github.com/tedsuo/rata"

	"cred-alert/config"
	"cred-alert/revok/api"
	"cred-alert/revok/api/middleware"
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
	logger.RegisterSink(lager.NewWriterSink(os.Stderr, lager.ERROR))
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

	redirectURL := mustGetEnv("REDIRECT_URL")
	sessionAuthKey := mustGetEnv("SESSION_AUTH_KEY")

	ssoService := getSSOService()

	authDomain := mustGetCredential(ssoService, "auth_domain")
	clientID := mustGetCredential(ssoService, "client_id")
	clientSecret := mustGetCredential(ssoService, "client_secret")

	oauthConfig := oauth2.Config{
		Endpoint: oauth2.Endpoint{
			AuthURL:  authDomain + "/oauth/authorize",
			TokenURL: authDomain + "/oauth/token",
		},
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
	}

	sessionStore := sessions.NewCookieStore([]byte(sessionAuthKey))

	handler, err := rata.NewRouter(web.Routes, rata.Handlers{
		web.Login:        api.NewLoginHandler(logger, sessionStore, oauthConfig),
		web.OAuth:        api.NewOAuthCallbackHandler(logger, sessionStore, oauthConfig, authDomain),
		web.Index:        middleware.NewAuthenticatedHandler(logger, sessionStore, oauthConfig, authDomain, api.NewIndexHandler(logger, indexLayout, revokClient)),
		web.Organization: middleware.NewAuthenticatedHandler(logger, sessionStore, oauthConfig, authDomain, api.NewOrganizationHandler(logger, organizationLayout, revokClient)),
		web.Repository:   middleware.NewAuthenticatedHandler(logger, sessionStore, oauthConfig, authDomain, api.NewRepositoryHandler(logger, repositoryLayout, revokClient)),
	})

	// Avoid memory leaks according to Gorilla documentation.
	handler = context.ClearHandler(handler)

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
	if val == "" {
		// TODO:
		err := fmt.Errorf("could not get value for key: %s", key)
		panic(err)
	}
	return val
}

func mustGetCredential(service cfenv.Service, key string) string {
	value, ok := service.CredentialString(key)
	if !ok || value == "" {
		// TODO:
		err := fmt.Errorf("could not get value for key: %s", key)
		panic(err)
	}

	return value
}

func getSSOService() cfenv.Service {
	appEnv, err := cfenv.Current()
	if err != nil {
		// TODO:
		panic(err)
	}

	services, err := appEnv.Services.WithNameUsingPattern("sso.*")
	if err != nil {
		// TODO:
		panic(err)
	}

	if len(services) != 1 {
		// TODO:
		panic("did not find exactly one sso service")
	}

	return services[0]
}

package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"code.cloudfoundry.org/lager"

	flags "github.com/jessevdk/go-flags"
	"github.com/pivotal-cf/paraphernalia/operate/admin"
	"github.com/pivotal-cf/paraphernalia/secure/tlsconfig"
	"github.com/pivotal-cf/paraphernalia/serve/grpcrunner"
	"github.com/robdimsdale/honeylager"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"

	"cred-alert/config"
	"cred-alert/db"
	"cred-alert/db/migrations"
	"cred-alert/revok"
	"cred-alert/revok/api"
	"cred-alert/revokpb"
)

var info = admin.ServiceInfo{
	Name:        "revok-api",
	Description: "A API service that provides counts of credentials for repositories.",
	Team:        "PCF Security Enablement",
}

func main() {
	var cfg *config.APIConfig
	var flagOpts config.APIOpts

	logger := lager.NewLogger("revok-api")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))
	logger.Info("starting")

	_, err := flags.Parse(&flagOpts)
	if err != nil {
		os.Exit(1)
	}

	bs, err := ioutil.ReadFile(string(flagOpts.ConfigFile))
	if err != nil {
		logger.Error("failed-to-open-config-file", err)
		os.Exit(1)
	}

	cfg, err = config.LoadAPIConfig(bs)
	if err != nil {
		logger.Error("failed-to-load-config-file", err)
		os.Exit(1)
	}

	errs := cfg.Validate()
	if errs != nil {
		for _, err := range errs {
			fmt.Println(err.Error())
		}
		os.Exit(1)
	}

	if cfg.Metrics.SentryDSN != "" {
		logger.RegisterSink(revok.NewSentrySink(cfg.Metrics.SentryDSN, cfg.Metrics.Environment))
	}

	if cfg.Metrics.HoneycombWriteKey != "" && cfg.Metrics.Environment != "" {
		s := honeylager.NewSink(cfg.Metrics.HoneycombWriteKey, cfg.Metrics.Environment, lager.DEBUG)
		defer s.Close()
		logger.RegisterSink(s)
	}

	dbCertificate, dbCaCertPool := loadCerts(
		cfg.MySQL.CertificatePath,
		cfg.MySQL.PrivateKeyPath,
		cfg.MySQL.PrivateKeyPassphrase,
		cfg.MySQL.CACertificatePath,
	)

	dbURI := db.NewDSN(
		cfg.MySQL.Username,
		cfg.MySQL.Password,
		cfg.MySQL.DBName,
		cfg.MySQL.Hostname,
		int(cfg.MySQL.Port),
		cfg.MySQL.ServerName,
		dbCertificate,
		dbCaCertPool,
	)

	database, err := migrations.LockDBAndMigrate(logger, "mysql", dbURI)
	if err != nil {
		log.Fatalf("db error: %s", err)
	}
	database.LogMode(false)

	repositoryRepository := db.NewRepositoryRepository(database)
	branchRepository := db.NewBranchRepository(database)

	certificate, caCertPool := loadCerts(
		cfg.Identity.CertificatePath,
		cfg.Identity.PrivateKeyPath,
		cfg.Identity.PrivateKeyPassphrase,
		cfg.Identity.CACertificatePath,
	)

	tlsConfig := tlsconfig.Build(
		tlsconfig.WithInternalServiceDefaults(),
		tlsconfig.WithIdentity(certificate),
	)

	handler := api.NewServer(logger, repositoryRepository, branchRepository)
	serverTLS := tlsConfig.Server(tlsconfig.WithClientAuthentication(caCertPool))
	grpcServer := grpcrunner.New(
		logger,
		fmt.Sprintf("%s:%d", cfg.API.BindIP, cfg.API.BindPort),
		func(server *grpc.Server) {
			revokpb.RegisterRevokAPIServer(server, handler)
		},
		grpc.Creds(credentials.NewTLS(serverTLS)),
	)

	members := []grouper.Member{
		{Name: "grpc-server", Runner: grpcServer},
		{Name: "debug", Runner: admin.Runner("6060", admin.WithInfo(info), admin.WithUptime())},
	}

	runner := sigmon.New(grouper.NewOrdered(os.Interrupt, members))
	err = <-ifrit.Invoke(runner).Wait()
	if err != nil {
		log.Fatalf("failed-to-start: %s", err)
	}
}

func loadCerts(certificatePath, privateKeyPath, privateKeyPassphrase, caCertificatePath string) (tls.Certificate, *x509.CertPool) {
	certificate, err := config.LoadCertificateFromFiles(
		certificatePath,
		privateKeyPath,
		privateKeyPassphrase,
	)
	if err != nil {
		log.Fatalln(err)
	}

	caCertPool, err := config.LoadCertificatePoolFromFiles(caCertificatePath)
	if err != nil {
		log.Fatalln(err)
	}

	return certificate, caCertPool
}

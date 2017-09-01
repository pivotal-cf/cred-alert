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
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"

	"cred-alert/config"
	"cred-alert/db"
	"cred-alert/db/migrations"
	"cred-alert/gitclient"
	"cred-alert/revok"
	"cred-alert/revok/api"
	"cred-alert/revokpb"
	"cred-alert/search"
)

var info = admin.ServiceInfo{
	Name:        "revok-api",
	Description: "A service allows lookup of (presence of) credentials in git repositories.",
	Team:        "PCF Security Enablement",
}

func main() {
	var cfg *config.WorkerConfig
	var flagOpts config.WorkerOpts

	logger := lager.NewLogger("revok-worker")
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

	cfg, err = config.LoadWorkerConfig(bs)
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

	debug := admin.Runner(
		"6061",
		admin.WithInfo(info),
		admin.WithUptime(),
	)

	members := []grouper.Member{
		{Name: "debug", Runner: debug},
	}

	looper := gitclient.NewLooper()
	searcher := search.NewSearcher(repositoryRepository, looper)

	fileLookup := gitclient.NewFileLookup()
	blobSearcher := search.NewBlobSearcher(repositoryRepository, fileLookup)
	handler := api.NewServer(logger, searcher, blobSearcher, repositoryRepository, branchRepository)

	serverTLS := tlsConfig.Server(tlsconfig.WithClientAuthentication(caCertPool))

	grpcServer := grpcrunner.New(
		logger,
		fmt.Sprintf("%s:%d", cfg.API.BindIP, cfg.API.BindPort),
		func(server *grpc.Server) {
			revokpb.RegisterRevokServer(server, handler)
		},
		grpc.Creds(credentials.NewTLS(serverTLS)),
	)

	members = append(members, grouper.Member{
		Name:   "grpc-server",
		Runner: grpcServer,
	})

	system := []grouper.Member{
		{
			Name:   "servers",
			Runner: grouper.NewParallel(os.Interrupt, members),
		},
	}

	runner := sigmon.New(grouper.NewOrdered(os.Interrupt, system))

	err = <-ifrit.Invoke(runner).Wait()
	if err != nil {
		log.Fatalf("failed-to-start: %s", err)
	}
}

func loadCerts(certificatePath, privateKeyPath, privateKeyPassphrase, caCertificatePath string) (tls.Certificate, *x509.CertPool) {
	certificate, err := config.LoadCertificate(
		certificatePath,
		privateKeyPath,
		privateKeyPassphrase,
	)
	if err != nil {
		log.Fatalln(err)
	}

	caCertPool, err := config.LoadCertificatePool(caCertificatePath)
	if err != nil {
		log.Fatalln(err)
	}

	return certificate, caCertPool
}

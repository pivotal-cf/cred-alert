package main

import (
	"fmt"
	"log"
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/jessevdk/go-flags"
	"github.com/pivotal-cf/paraphernalia/operate/admin"
	"github.com/pivotal-cf/paraphernalia/secure/tlsconfig"
	"github.com/pivotal-cf/paraphernalia/serve/grpcrunner"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"cred-alert/cmdflag"
	"cred-alert/config"
	"cred-alert/gitclient"
	"cred-alert/metrics"
	"cred-alert/revok"
	"rolodex"
	"rolodex/rolodexpb"
)

type RolodexOpts struct {
	ConfigFile cmdflag.FileFlag `long:"config-file" description:"path to config file" required:"true" value-name:"PATH"`
}

func main() {
	logger := lager.NewLogger("rolodexd")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.INFO))
	logger.Info("starting")

	cfg := loadConfig()

	if cfg.Metrics.SentryDSN != "" {
		logger.RegisterSink(revok.NewSentrySink(cfg.Metrics.SentryDSN, cfg.Metrics.Environment))
	}

	certificate, err := config.LoadCertificate(
		cfg.RPC.CertificatePath,
		cfg.RPC.PrivateKeyPath,
		cfg.RPC.PrivateKeyPassphrase,
	)
	if err != nil {
		log.Fatalln(err)
	}

	clientCertPool, err := config.LoadCertificatePool(cfg.RPC.CACertificatePath)
	if err != nil {
		log.Fatalln(err)
	}

	repository := rolodex.NewTeamRepository(logger, cfg.RepositoryPath)
	emitter := metrics.BuildEmitter(cfg.Metrics.DatadogAPIKey, cfg.Metrics.Environment)
	handler := rolodex.NewHandler(logger, repository, emitter)
	gitClient := gitclient.New(cfg.GitHub.PrivateKeyPath, cfg.GitHub.PublicKeyPath)
	syncer := rolodex.NewSyncer(logger, emitter, cfg.RepositoryURL, cfg.RepositoryPath, gitClient, repository)

	tlsConfig := tlsconfig.Build(
		tlsconfig.WithPivotalDefaults(),
		tlsconfig.WithIdentity(certificate),
	).Server(tlsconfig.WithClientAuthentication(clientCertPool))

	grpcServer := grpcrunner.NewGRPCServer(
		logger,
		fmt.Sprintf("%s:%d", cfg.RPC.BindIP, cfg.RPC.BindPort),
		func(server *grpc.Server) {
			rolodexpb.RegisterRolodexServer(server, handler)
		},
		grpc.Creds(credentials.NewTLS(tlsConfig)),
	)

	info := admin.ServiceInfo{
		Name:        "rolodex",
		Description: "A service providing information about internal teams.",
		Team:        "PCF Security Enablement",
	}
	debug := admin.Runner("6060", admin.WithInfo(info))

	members := []grouper.Member{
		{"api", grpcServer},
		{"debug", debug},
		{"syncer", revok.Schedule(logger, "@every 5m", func() {
			syncer.Sync()
		})},
	}

	runner := sigmon.New(grouper.NewParallel(os.Interrupt, members))
	err = <-ifrit.Invoke(runner).Wait()
	if err != nil {
		logger.Fatal("failed-to-start", err)
	}
}

func loadConfig() rolodex.Config {
	var flagOpts RolodexOpts

	_, err := flags.Parse(&flagOpts)
	if err != nil {
		os.Exit(1)
	}

	cfg, err := rolodex.LoadConfig(string(flagOpts.ConfigFile))
	if err != nil {
		log.Fatalln("failed to load config", err)
	}

	errs := cfg.Validate()
	if errs != nil {
		for _, err := range errs {
			fmt.Println(err.Error())
		}
		os.Exit(1)
	}

	return *cfg
}

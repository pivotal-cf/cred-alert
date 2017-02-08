package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"net/http/pprof"
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/jessevdk/go-flags"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
	"golang.org/x/net/trace"
	"google.golang.org/grpc"

	"cred-alert/cmdflag"
	"cred-alert/config"
	"cred-alert/gitclient"
	"cred-alert/revok"
	"red/redrunner"
	"rolodex"
	"rolodex/rolodexpb"
	"cred-alert/metrics"
)

type RolodexOpts struct {
	ConfigFile cmdflag.FileFlag `long:"config-file" description:"path to config file" required:"true" value-name:"PATH"`
}

func main() {
	logger := lager.NewLogger("rolodexd")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.INFO))

	logger.Debug("starting")

	var flagOpts RolodexOpts
	_, err := flags.Parse(&flagOpts)
	if err != nil {
		os.Exit(1)
	}

	cfg, err := rolodex.LoadRolodexConfig(string(flagOpts.ConfigFile))
	if err != nil {
		logger.Fatal("failed-to-load-config", err)
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

	grpcServer := redrunner.NewGRPCServer(
		logger,
		fmt.Sprintf("%s:%d", cfg.RPC.BindIP, cfg.RPC.BindPort),
		&tls.Config{
			ClientAuth:   tls.RequireAndVerifyClientCert,
			Certificates: []tls.Certificate{certificate},
			ClientCAs:    clientCertPool,
		},
		func(server *grpc.Server) {
			rolodexpb.RegisterRolodexServer(server, handler)
		},
	)

	members := []grouper.Member{
		{"api", grpcServer},
		{"debug", http_server.New("127.0.0.1:6060", debugHandler())},
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

func debugHandler() http.Handler {
	debugRouter := http.NewServeMux()
	debugRouter.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
	debugRouter.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
	debugRouter.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
	debugRouter.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
	debugRouter.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))

	debugRouter.HandleFunc("/debug/requests", func(w http.ResponseWriter, req *http.Request) {
		any, sensitive := trace.AuthRequest(req)
		if !any {
			http.Error(w, "not allowed", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		trace.Render(w, req, sensitive)
	})

	debugRouter.HandleFunc("/debug/events", func(w http.ResponseWriter, req *http.Request) {
		any, sensitive := trace.AuthRequest(req)
		if !any {
			http.Error(w, "not allowed", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		trace.RenderEvents(w, req, sensitive)
	})

	return debugRouter
}

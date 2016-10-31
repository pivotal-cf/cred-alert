package revok

import (
	"cred-alert/revokpb"
	"crypto/tls"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"code.cloudfoundry.org/lager"

	"github.com/tedsuo/ifrit"
)

type grpcServer struct {
	logger      lager.Logger
	listenAddr  string
	revokServer RevokServer
	tlsConfig   *tls.Config
}

func NewGRPCServer(
	logger lager.Logger,
	listenAddr string,
	revokServer RevokServer,
	tlsConfig *tls.Config,
) ifrit.Runner {
	return &grpcServer{
		logger:      logger,
		listenAddr:  listenAddr,
		revokServer: revokServer,
		tlsConfig:   tlsConfig,
	}
}

func (s *grpcServer) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := s.logger.Session("grpc-server")

	lis, err := net.Listen("tcp", s.listenAddr)
	if err != nil {
		return err
	}

	serverOption := grpc.Creds(credentials.NewTLS(s.tlsConfig))
	server := grpc.NewServer(serverOption)
	revokpb.RegisterRevokServer(server, s.revokServer)

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Serve(lis)
	}()

	close(ready)

	logger.Info("started")

	select {
	case err = <-errCh:
		return err
	case <-signals:
		server.GracefulStop()
	}

	logger.Info("exiting")

	return nil
}

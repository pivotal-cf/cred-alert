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
	logger     lager.Logger
	listenAddr string
	server     Server
	tlsConfig  *tls.Config
}

func NewGRPCServer(
	logger lager.Logger,
	listenAddr string,
	server Server,
	tlsConfig *tls.Config,
) ifrit.Runner {
	return &grpcServer{
		logger:     logger,
		listenAddr: listenAddr,
		server:     server,
		tlsConfig:  tlsConfig,
	}
}

func (s *grpcServer) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := s.logger.Session("grpc-server")

	lis, err := net.Listen("tcp", s.listenAddr)
	if err != nil {
		return err
	}

	serverOption := grpc.Creds(credentials.NewTLS(s.tlsConfig))
	grpcServer := grpc.NewServer(serverOption)
	revokpb.RegisterRevokServer(grpcServer, s.server)

	errCh := make(chan error, 1)
	go func() {
		errCh <- grpcServer.Serve(lis)
	}()

	close(ready)

	logger.Info("started")

	select {
	case err = <-errCh:
		return err
	case <-signals:
		grpcServer.GracefulStop()
	}

	logger.Info("exiting")

	return nil
}

package redrunner

import (
	"crypto/tls"
	"net"
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/ifrit"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type GRPCRegisterFunc func(server *grpc.Server)

type grpcServer struct {
	logger lager.Logger

	listenAddr string
	tlsConfig  *tls.Config

	registerFunc GRPCRegisterFunc
}

func NewGRPCServer(
	logger lager.Logger,
	listenAddr string,
	tlsConfig *tls.Config,
	registerFunc GRPCRegisterFunc,
) ifrit.Runner {
	return &grpcServer{
		logger:       logger,
		listenAddr:   listenAddr,
		tlsConfig:    tlsConfig,
		registerFunc: registerFunc,
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
	s.registerFunc(grpcServer)

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

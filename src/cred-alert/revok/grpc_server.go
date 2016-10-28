package revok

import (
	"cred-alert/revokpb"
	"net"
	"os"

	"google.golang.org/grpc"

	"code.cloudfoundry.org/lager"

	"github.com/tedsuo/ifrit"
)

type grpcServer struct {
	logger      lager.Logger
	listenAddr  string
	revokServer RevokServer
}

func NewGRPCServer(
	logger lager.Logger,
	listenAddr string,
	revokServer RevokServer,
) ifrit.Runner {
	return &grpcServer{
		logger:      logger,
		listenAddr:  listenAddr,
		revokServer: revokServer,
	}
}

func (r *grpcServer) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := r.logger.Session("grpc-server")

	lis, err := net.Listen("tcp", r.listenAddr)
	if err != nil {
		return err
	}

	close(ready)

	logger.Info("started")

	s := grpc.NewServer()
	revokpb.RegisterRevokServer(s, r.revokServer)

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.Serve(lis)
	}()

	select {
	case err = <-errCh:
		return err
	case <-signals:
		s.GracefulStop()
	}

	logger.Info("exiting")

	return nil
}

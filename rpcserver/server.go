package rpcserver

import (
	"carlaKC/boltnd/offersrpc"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/lightninglabs/lndclient"
)

// Compile time check that this server implements our grpc server.
var _ offersrpc.OffersServer = (*Server)(nil)

// Server implements our offersrpc server.
type Server struct {
	started int32 // to be used atomically
	stopped int32 // to be used atomically

	cfg *lndclient.LndServicesConfig

	// lnd provides a connection to lnd's other grpc servers. Since we need
	// to connect to the grpc servers, this can't be done while we're busy
	// setting up ourselves as a sub-server. Consequently, this value will
	// only be non-nil once Start() has been called.
	lnd *lndclient.GrpcLndServices

	offersrpc.UnimplementedOffersServer
}

// NewServer creates an offers server.
func NewServer(cfg *lndclient.LndServicesConfig) (*Server, error) {
	return &Server{
		cfg: cfg,
	}, nil
}

// Start starts the offers server.
func (s *Server) Start() error {
	if !atomic.CompareAndSwapInt32(&s.started, 0, 1) {
		return errors.New("server already started")
	}

	// Setup our lnd grpc client.
	var err error
	s.lnd, err = lndclient.NewLndServices(s.cfg)
	if err != nil {
		return err
	}

	return nil
}

// Stop shuts down the server.
func (s *Server) Stop() error {
	if !atomic.CompareAndSwapInt32(&s.stopped, 0, 1) {
		return fmt.Errorf("server already stopped")
	}

	// Shut down our lnd grpc client.
	s.lnd.Close()

	return nil
}

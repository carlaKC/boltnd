package rpcserver

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/carlakc/boltnd/offersrpc"
	"github.com/carlakc/boltnd/onionmsg"
	"github.com/lightninglabs/lndclient"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Compile time check that this server implements our grpc server.
var _ offersrpc.OffersServer = (*Server)(nil)

// Server implements our offersrpc server.
type Server struct {
	started int32 // to be used atomically
	stopped int32 // to be used atomically

	// lnd provides a connection to lnd's other grpc servers. Since we need
	// to connect to the grpc servers, this can't be done while we're busy
	// setting up ourselves as a sub-server. Consequently, this value will
	// only be non-nil once Start() has been called.
	lnd *lndclient.LndServices

	// onionMsgr manages sending and receipt of onion messages with peers.
	// As with the lnd instance above, the messenger is only created once
	// Start() has been called.
	onionMsgr onionmsg.OnionMessenger

	// ready is closed once the server is fully set up and ready to operate.
	// This is required because we only receive our lnd dependency on
	// Start().
	ready chan (struct{})

	offersrpc.UnimplementedOffersServer
}

// NewServer creates an offers server.
func NewServer() (*Server, error) {
	return &Server{
		ready: make(chan struct{}),
	}, nil
}

// Start starts the offers server.
func (s *Server) Start(lnd *lndclient.LndServices) error {
	if !atomic.CompareAndSwapInt32(&s.started, 0, 1) {
		return errors.New("server already started")
	}

	log.Info("Starting rpc server")
	s.lnd = lnd
	s.onionMsgr = onionmsg.NewOnionMessenger(lnd.Client)

	close(s.ready)

	return nil
}

// Stop shuts down the server.
func (s *Server) Stop() error {
	if !atomic.CompareAndSwapInt32(&s.stopped, 0, 1) {
		return fmt.Errorf("server already stopped")
	}

	log.Info("Stopping rpc server")
	defer log.Info("Stopped rpc server")

	return nil
}

// waitForReady blocks until the server is initialized or the context provided
// is cancelled.
func (s *Server) waitForReady(ctx context.Context) error { // nolint: unused
	select {
	case <-s.ready:
		return nil

	case <-ctx.Done():
		return status.Error(codes.Unavailable, "server not ready")
	}
}

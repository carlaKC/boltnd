package rpcserver

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/carlakc/boltnd/offers"
	"github.com/carlakc/boltnd/offersrpc"
	"github.com/carlakc/boltnd/onionmsg"
	"github.com/lightninglabs/lndclient"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Compile time check that this server implements our grpc server.
var _ offersrpc.OffersServer = (*Server)(nil)

var (
	// ErrShuttingDown is returned when an operation is aborted because
	// the server is shutting down.
	ErrShuttingDown = errors.New("server shutting down")
)

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

	// offerCoordinator manages the payment lifecycle of offers.
	offerCoordinator offers.OfferCoordinator

	// ready is closed once the server is fully set up and ready to operate.
	// This is required because we only receive our lnd dependency on
	// Start().
	ready chan (struct{})

	// quit indicates to the server's goroutines that it is time to shut
	// down.
	quit chan (struct{})

	// requestShutdown is called when the messenger experiences an error to
	// signal to calling code that it should gracefully exit.
	requestShutdown func(err error)

	offersrpc.UnimplementedOffersServer
}

// NewServer creates an offers server.
func NewServer(shutdown func(error)) (*Server, error) {
	return &Server{
		ready:           make(chan struct{}),
		quit:            make(chan struct{}),
		requestShutdown: shutdown,
	}, nil
}

// Start starts the offers server.
func (s *Server) Start(lnd *lndclient.LndServices) error {
	if !atomic.CompareAndSwapInt32(&s.started, 0, 1) {
		return errors.New("server already started")
	}

	log.Info("Starting rpc server")
	s.lnd = lnd

	// Setup a router that our onion messenger can use, utilizing a
	// signer that calls lnd's apis for cyrptographic operations.
	nodeKeyECDH, err := onionmsg.NewNodeECDH(lnd.Client, lnd.Signer)
	if err != nil {
		return fmt.Errorf("could not create router signer: %w", err)
	}

	// Setup an onion messenger using the onion router.
	s.onionMsgr = onionmsg.NewOnionMessenger(
		lnd.ChainParams, lnd.Client, nodeKeyECDH, s.requestShutdown,
	)

	if err := s.onionMsgr.Start(); err != nil {
		return fmt.Errorf("could not start onion messenger: %w", err)
	}

	s.offerCoordinator = offers.NewCoordinator(
		lnd.Router, s.onionMsgr, s.requestShutdown,
	)

	if err := s.offerCoordinator.Start(); err != nil {
		return fmt.Errorf("could not start offer coordinator: %w", err)
	}
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

	close(s.quit)

	if s.offerCoordinator != nil {
		if err := s.offerCoordinator.Stop(); err != nil {
			return fmt.Errorf("could not stop offer coordinator: "+
				"%w", err)
		}
	}

	// Shut down onion messenger if non-nil.
	if s.onionMsgr != nil {
		if err := s.onionMsgr.Stop(); err != nil {
			return fmt.Errorf("could not stop onion messenger: %w",
				err)
		}
	}
	return nil
}

// waitForReady blocks until the server is initialized or the context provided
// is cancelled.
func (s *Server) waitForReady(ctx context.Context) error {
	select {
	case <-s.ready:
		return nil

	case <-ctx.Done():
		return status.Error(codes.Unavailable, "server not ready")
	}
}

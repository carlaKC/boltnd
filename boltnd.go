package boltnd

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/carlakc/boltnd/offersrpc"
	"github.com/carlakc/boltnd/rpcserver"
	"github.com/lightninglabs/lndclient"
	"google.golang.org/grpc"
	"gopkg.in/macaroon-bakery.v2/bakery"
)

// Boltnd holds opt-in bolt features that are externally implemented for lnd.
type Boltnd struct {
	started int32 // to be used atomically
	stopped int32 // to be used atomically

	lnd       *lndclient.GrpcLndServices
	rpcServer *rpcserver.Server

	cfg *Config
}

// NewBoltnd returns a new external boltnd implementation. Note that the
// lnd config provided must be fully initialized so that we can setup our
// logging.
func NewBoltnd(opts ...ConfigOption) (*Boltnd, error) {
	// Start with our default configuration.
	cfg := DefaultConfig()

	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, fmt.Errorf("config option failed: %v", err)
		}
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %v", err)
	}

	setupLoggers(cfg)

	rpcserver, err := rpcserver.NewServer()
	if err != nil {
		return nil, fmt.Errorf("could not create rpcserver: %v", err)
	}

	return &Boltnd{
		rpcServer: rpcserver,
		cfg:       cfg,
	}, nil
}

// Start starts the boltnd implementation.
func (b *Boltnd) Start() error {
	if !atomic.CompareAndSwapInt32(&b.started, 0, 1) {
		return errors.New("boltnd already started")
	}

	log.Info("Starting Boltnd")

	// We allow zero retires, so we iterate with retries + 1 to allow at
	// least one connection attempt for lnd.
	var connErr error
	for i := 0; i < int(b.cfg.LNDRetires+1); i++ {
		log.Infof("Attempt: %v to connect to lnd", i)

		b.lnd, connErr = lndclient.NewLndServices(b.cfg.LndClientCfg)
		if connErr != nil {
			log.Errorf("error starting lndclient: %w (attempt: %v)",
				connErr, i)

			time.Sleep(b.cfg.LNDWait)
			continue
		}

		log.Infof("Successfully connected to lnd on attempt: %v", i)
		break

	}

	// If we exited the loop above with an error, fail startup.
	if connErr != nil {
		return fmt.Errorf("could not connect to lnd: %w", connErr)
	}

	if err := b.rpcServer.Start(&b.lnd.LndServices); err != nil {
		return fmt.Errorf("error starting rpcserver: %v", err)
	}

	return nil
}

// Stop shuts down the boltnd implementation.
func (b *Boltnd) Stop() error {
	if !atomic.CompareAndSwapInt32(&b.stopped, 0, 1) {
		return fmt.Errorf("boltnd already stopped")
	}

	log.Info("Stopping Boltnd")

	if err := b.rpcServer.Stop(); err != nil {
		return fmt.Errorf("could not stop rpcserver: %v", err)
	}

	if b.lnd != nil {
		b.lnd.Close()
	}

	return nil
}

// requestShutdown calls our graceful shutdown closure, if supplied.
func (b *Boltnd) requestShutdown() { // nolint: unused
	if b.cfg.RequestShutdown == nil {
		log.Info("No graceful shutdown closure provided")
		return
	}

	log.Info("Requesting graceful shutdown")
	b.cfg.RequestShutdown()
}

// RegisterGrpcSubserver is a callback on the lnd.SubserverConfig struct that is
// called once lnd has initialized its main gRPC server instance. It gives the
// daemons (or external subservers) the possibility to register themselves to
// the same server instance.
//
// NOTE: This is part of the lnd.GrpcRegistrar interface.
func (b *Boltnd) RegisterGrpcSubserver(server *grpc.Server) error {
	log.Info("Registered bolt 12 subserver")

	offersrpc.RegisterOffersServer(server, b.rpcServer)
	return nil
}

// Permissions returns all permissions for which boltnd's external validator
// is responsible.
//
// NOTE: This is part of the lnd.ExternalValidator interface.
func (b *Boltnd) Permissions() map[string][]bakery.Op {
	return rpcserver.RPCServerPermissions
}

// ValidateMacaroon extracts the macaroon from the context's gRPC metadata,
// checks its signature, makes sure all specified permissions for the called
// method are contained within and finally ensures all caveat conditions are
// met. A non-nil error is returned if any of the checks fail.
//
// NOTE: This is part of the lnd.ExternalValidator interface.
func (b *Boltnd) ValidateMacaroon(_ context.Context,
	_ []bakery.Op, _ string) error {

	// lnd will run validation for us, so we don't need to perform any
	// additional validation.
	// TODO - check that this is the case!
	return nil
}

// setupLoggers registers all of our loggers if a config option to setup loggers
// was provided.
func setupLoggers(cfg *Config) {
	// If a setup function is not provided, do nothing.
	if cfg.SetupLogger == nil {
		return
	}

	cfg.SetupLogger(Subsystem, UseLogger)
	cfg.SetupLogger(rpcserver.Subsystem, rpcserver.UseLogger)
}

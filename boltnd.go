package boltnd

import (
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/carlakc/boltnd/offersrpc"
	"github.com/carlakc/boltnd/rpcserver"
	"google.golang.org/grpc"
)

// Boltnd holds opt-in bolt features that are externally implemented for lnd.
type Boltnd struct {
	started int32 // to be used atomically
	stopped int32 // to be used atomically

	rpcServer *rpcserver.Server

	cfg *Config
}

// NewBoltnd returns a new external boltnd implementation. Note that the
// lnd config provided must be fully initialized so that we can setup our
// logging.
func NewBoltnd(opts ...ConfigOption) (*Boltnd, error) {
	// Start with an empty config and apply our functional options.
	cfg := &Config{}

	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, fmt.Errorf("config option failed: %v", err)
		}
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %v", err)
	}

	setupLoggers(cfg)

	rpcserver, err := rpcserver.NewServer(cfg.LndClientCfg)
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

	if err := b.rpcServer.Start(); err != nil {
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

	return nil
}

// requestShutdown calls our graceful shutdown closure, if supplied.
func (b *Boltnd) requestShutdown() {
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

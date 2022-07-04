package boltnd

import (
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/carlakc/boltnd/offersrpc"
	"github.com/carlakc/boltnd/rpcserver"
	"github.com/lightninglabs/lndclient"
	"github.com/lightningnetwork/lnd"
	"github.com/lightningnetwork/lnd/build"
	"github.com/lightningnetwork/lnd/lnrpc/verrpc"
	"github.com/lightningnetwork/lnd/signal"
	"google.golang.org/grpc"
)

// Boltnd holds opt-in bolt features that are externally implemented for lnd.
type Boltnd struct {
	started int32 // to be used atomically
	stopped int32 // to be used atomically

	rpcServer *rpcserver.Server
}

// NewBoltnd returns a new external boltnd implementation. Note that the
// lnd config provided must be fully initialized so that we can setup our
// logging.
func NewBoltnd(cfg *lnd.Config,
	interceptor signal.Interceptor) (*Boltnd, error) {

	// Register our logger as a sublogger in lnd.
	setupLoggers(cfg.LogWriter, interceptor)

	lndClientCfg, err := lndClientCfg(cfg)
	if err != nil {
		return nil, err
	}

	rpcserver, err := rpcserver.NewServer(lndClientCfg)
	if err != nil {
		return nil, err
	}

	return &Boltnd{
		rpcServer: rpcserver,
	}, nil
}

// Start starts the boltnd implementation.
func (b *Boltnd) Start() error {
	if !atomic.CompareAndSwapInt32(&b.started, 0, 1) {
		return errors.New("boltnd already started")
	}

	log.Info("Starting Boltnd")

	if err := b.rpcServer.Start(); err != nil {
		return err
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
		return err
	}

	return nil
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

// lndClientCfg extracts a lndclient config from the top level lnd config.
func lndClientCfg(cfg *lnd.Config) (*lndclient.LndServicesConfig, error) {
	if len(cfg.RPCListeners) < 1 {
		return nil, errors.New("at least one rpc listener required")
	}

	// Setup a config to connect to lnd from the top level config passed in.
	lndCfg := &lndclient.LndServicesConfig{
		LndAddress:         cfg.RPCListeners[0].String(),
		CustomMacaroonPath: cfg.AdminMacPath,
		TLSPath:            cfg.TLSCertPath,
		CheckVersion: &verrpc.Version{
			AppMajor: 0,
			AppMinor: 15,
			AppPatch: 0,
			BuildTags: []string{
				"signrpc", "walletrpc", "chainrpc",
				"invoicesrpc", "bolt12",
			},
		},
		BlockUntilChainSynced: true,
		BlockUntilUnlocked:    true,
	}

	switch {
	case cfg.Bitcoin.MainNet:
		lndCfg.Network = lndclient.NetworkMainnet

	case cfg.Bitcoin.TestNet3:
		lndCfg.Network = lndclient.NetworkTestnet

	case cfg.Bitcoin.RegTest:
		lndCfg.Network = lndclient.NetworkRegtest

	default:
		return nil, fmt.Errorf("only bitcoin mainnet /testnet / " +
			"regtest supported")
	}

	return lndCfg, nil
}

// setupLoggers registers all of our loggers as subloggers with lnd.
func setupLoggers(root *build.RotatingLogWriter,
	interceptor signal.Interceptor) {

	lnd.AddSubLogger(root, Subsystem, interceptor, UseLogger)
	lnd.AddSubLogger(
		root, rpcserver.Subsystem, interceptor,
		rpcserver.UseLogger,
	)
}

package boltnd

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/lightninglabs/lndclient"
	"github.com/lightningnetwork/lnd"
	"github.com/lightningnetwork/lnd/build"
	"github.com/lightningnetwork/lnd/lnrpc/verrpc"
	"github.com/lightningnetwork/lnd/signal"
)

const (
	// DefaultLNDRetries is the default number of times we retry connecting
	// to lnd before erroring out.
	DefaultLNDRetries = 4

	// DefaultLNDWait is the default amount of time we backoff between
	// lnd connection attempts.
	DefaultLNDWait = time.Second * 10
)

// MinimumLNDVersion is the minimum lnd version and set of build tags required.
var MinimumLNDVersion = &verrpc.Version{
	AppMajor: 0,
	AppMinor: 15,
	AppPatch: 0,
	BuildTags: []string{
		"signrpc", "walletrpc", "chainrpc", "invoicesrpc",
		// TODO(carla): lnd's itests don't set this build tag, but
		// running vanilla lnd does. Figure out why and require this
		// tag again.
		// "bolt12",
	},
}

// Config contains the configuration required for boltnd.
type Config struct {
	// LndClientCfg provides configuration
	LndClientCfg *lndclient.LndServicesConfig

	// SetupLogger is used to register our loggers with a top level logger.
	SetupLogger func(prefix string, register LogRegistration)

	// RequestShutdown is an optional closure to request clean shutdown
	// from the calling entity if the boltnd instance errors out.
	RequestShutdown func()

	// LNDRetires is the number of times we try to connect to lnd's grpc
	// server (with backoff set by LNDWait) before exiting with an error.
	// This value should be set to 0 if we want to immediately exit if
	// we can't successfully connect.
	LNDRetires uint8

	// LNDWait is the amount of time to wait between retries to connect to
	// lnd's grpc server.
	LNDWait time.Duration
}

// DefaultConfig returns a default config.
func DefaultConfig() *Config {
	return &Config{
		// Our default lndclient config uses the default values as set
		// in lnd. These values aren't all exported in lnd, so we use
		// the defaults as in 0.15.
		LndClientCfg: &lndclient.LndServicesConfig{
			LndAddress: "localhost:10009",
			MacaroonDir: filepath.Join(
				lnd.DefaultLndDir, "data", "chain", "mainnet",
			),

			TLSPath: filepath.Join(
				lnd.DefaultLndDir, "tls.cert",
			),
			CheckVersion:          MinimumLNDVersion,
			BlockUntilChainSynced: true,
			BlockUntilUnlocked:    true,
			Network:               "mainnet",
		},
		LNDRetires: DefaultLNDRetries,
		LNDWait:    DefaultLNDWait,
	}
}

// Validate ensures that we have all the required config values set.
func (c *Config) Validate() error {
	if c.LndClientCfg == nil {
		return errors.New("lnd client config required")
	}

	if c.LndClientCfg.CheckVersion == nil {
		return errors.New("lnd check version required")
	}

	// Check that we at least have our minimum build tags and version.
	if err := lndclient.AssertVersionCompatible(
		c.LndClientCfg.CheckVersion, MinimumLNDVersion,
	); err != nil {
		return err
	}

	if c.LNDWait < 0 {
		return fmt.Errorf("wait: %v must be > 0", c.LNDWait)
	}

	return nil
}

// ConfigOption is the function signature used for functional options that
// update config.
type ConfigOption func(*Config) error

// OptionLNDConfig returns a functional option that will use lnd's internal
// config struct to create the lndclient config for our lndclient config.
func OptionLNDConfig(cfg *lnd.Config) ConfigOption {
	return func(c *Config) error {

		if len(cfg.RPCListeners) < 1 {
			return errors.New("at least one rpc listener " +
				"required")
		}

		// Setup our lndclient config to connect to lnd from the top
		// level config passed in.
		c.LndClientCfg = &lndclient.LndServicesConfig{
			LndAddress:            cfg.RPCListeners[0].String(),
			CustomMacaroonPath:    cfg.AdminMacPath,
			TLSPath:               cfg.TLSCertPath,
			CheckVersion:          MinimumLNDVersion,
			BlockUntilChainSynced: true,
			BlockUntilUnlocked:    true,
		}

		switch {
		case cfg.Bitcoin.MainNet:
			c.LndClientCfg.Network = lndclient.NetworkMainnet

		case cfg.Bitcoin.TestNet3:
			c.LndClientCfg.Network = lndclient.NetworkTestnet

		case cfg.Bitcoin.RegTest:
			c.LndClientCfg.Network = lndclient.NetworkRegtest

		default:
			return fmt.Errorf("only bitcoin mainnet/testnet/" +
				"regtest supported")
		}

		return nil
	}
}

// OptionLNDClient sets the lnd client config in our top level config.
func OptionLNDClient(lndClientCfg *lndclient.LndServicesConfig) ConfigOption {
	return func(c *Config) error {
		c.LndClientCfg = lndClientCfg
		return nil
	}
}

// OptionLNDLogger uses lnd's root logger and interceptor to register our logs.
func OptionLNDLogger(root *build.RotatingLogWriter,
	interceptor signal.Interceptor) ConfigOption {

	return func(c *Config) error {
		c.SetupLogger = func(prefix string, r LogRegistration) {
			lnd.AddSubLogger(root, prefix, interceptor, r)
		}

		return nil
	}
}

// OptionSetupLogger sets the setup logger function in our config.
func OptionSetupLogger(setup func(string, LogRegistration)) ConfigOption {
	return func(c *Config) error {
		c.SetupLogger = setup
		return nil
	}
}

// OptionRequestShutdown provides a closure that will gracefully shutdown the
// calling code if boltnd exits with an error.
func OptionRequestShutdown(s func()) ConfigOption {
	return func(c *Config) error {
		c.RequestShutdown = s
		return nil
	}
}

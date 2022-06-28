package boltnd

import (
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/lightningnetwork/lnd"
	"github.com/lightningnetwork/lnd/signal"
)

// Bolt12Ext holds a bolt 12 implementation that is external to lnd.
type Bolt12Ext struct {
	started int32 // to be used atomically
	stopped int32 // to be used atomically
}

// NewBolt12Ext returns a new external bolt 12 implementation. Note that the
// lnd config provided must be fully initialized so that we can setup our
// logging.
func NewBolt12Ext(cfg *lnd.Config,
	interceptor signal.Interceptor) (*Bolt12Ext, error) {

	// Register our logger as a sublogger in lnd.
	lnd.AddSubLogger(cfg.LogWriter, Subsystem, interceptor, UseLogger)

	return &Bolt12Ext{}, nil
}

// Start starts the bolt 12 implementation.
func (b *Bolt12Ext) Start() error {
	if !atomic.CompareAndSwapInt32(&b.started, 0, 1) {
		return errors.New("impl already started")
	}

	return nil
}

// Stop shuts down the bolt 12 implementation.
func (b *Bolt12Ext) Stop() error {
	if !atomic.CompareAndSwapInt32(&b.stopped, 0, 1) {
		return fmt.Errorf("impl already stopped")
	}

	return nil
}

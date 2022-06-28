package boltnd

import (
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/lightningnetwork/lnd"
)

// Bolt12Ext holds a bolt 12 implementation that is external to lnd.
type Bolt12Ext struct {
	started int32 // to be used atomically
	stopped int32 // to be used atomically
}

// NewBolt12Ext returns a new external bolt 12 implementation.
func NewBolt12Ext(cfg lnd.Config) *Bolt12Ext {
	return &Bolt12Ext{}
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

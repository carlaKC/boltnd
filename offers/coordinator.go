package offers

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/carlakc/boltnd/lnwire"
)

var (
	// ErrShuttingDown is returned when the coordinator exits.
	ErrShuttingDown = errors.New("coordinator shutting down")

	// ErrOfferIDIncorrect is returned if an invoice's ID doesn't match
	// our offer merkle root.
	ErrOfferIDIncorrect = errors.New("invoice offer ID does not match " +
		"original offer")

	// ErrNodeIDIncorrect is returned when an invoice's node ID doesn't
	// match the original offer.
	ErrNodeIDIncorrect = errors.New("invoice node ID does not match " +
		"original offer")

	// ErrDescriptionIncorrect is returned when an invoice's description
	// doesn't match the original offer.
	ErrDescriptionIncorrect = errors.New("invoice description does not " +
		"match original offer")

	// ErrAmountIncorrect is returned if an invoice's amount is less than
	// the minimum set by an offer.
	ErrAmountIncorrect = errors.New("invoice amount < offer minimum")
)

// Coordinator manages the exchange of offer-related messages and payment of
// invoices.
type Coordinator struct {
	started int32 // to be used atomically
	stopped int32 // to be used atomically

	// lnd provides the lnd apis required for offers.
	lnd LNDOffers

	// requestShutdown is called when the messenger experiences an error to
	// signal to calling code that it should gracefully exit.
	requestShutdown func(err error)

	wg   sync.WaitGroup
	quit chan struct{}
}

// NewCoordinator creates a new offer coordinator.
func NewCoordinator(lnd LNDOffers,
	requestShutdown func(err error)) *Coordinator {

	return &Coordinator{
		lnd:             lnd,
		requestShutdown: requestShutdown,
		quit:            make(chan struct{}),
	}
}

// Start runs the coordinator.
func (c *Coordinator) Start() error {
	if !atomic.CompareAndSwapInt32(&c.started, 0, 1) {
		return errors.New("coordinator already started")
	}

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()

		err := c.handleOffers()
		if err != nil && err != ErrShuttingDown {
			c.requestShutdown(err)
		}
	}()

	return nil
}

// Stop shuts down the coordinator.
func (c *Coordinator) Stop() error {
	if !atomic.CompareAndSwapInt32(&c.stopped, 0, 1) {
		return fmt.Errorf("coordinator already stopped")
	}

	close(c.quit)
	c.wg.Wait()

	return nil
}

// handleOffers is the main goroutine that handles offer exchanges.
func (c *Coordinator) handleOffers() error {
	for {
		select {
		case <-c.quit:
			return ErrShuttingDown
		}
	}
}

// validateExchange validates that the messages that form part of a single
// offer exchange are valid.
func validateExchange(o *lnwire.Offer, i *lnwire.Invoice) error {
	if !bytes.Equal(o.MerkleRoot[:], i.OfferID[:]) {
		return fmt.Errorf("%w: %x merkle root, %x offer id",
			ErrOfferIDIncorrect, o.MerkleRoot, i.OfferID)
	}

	if o.NodeID != i.NodeID {
		return fmt.Errorf("%w: offer: %v, invoice: %v",
			ErrNodeIDIncorrect, o.NodeID, i.NodeID)
	}

	if o.Description != i.Description {
		return fmt.Errorf("%w: offer: %v, invoice: %v",
			ErrDescriptionIncorrect, o.Description, i.Description)
	}

	if i.Amount < o.MinimumAmount {
		return fmt.Errorf("%w: %v < %v", ErrAmountIncorrect, i.Amount,
			o.MinimumAmount)
	}

	return nil
}

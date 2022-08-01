package offers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/carlakc/boltnd/lnwire"
	"github.com/lightninglabs/lndclient"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/lightningnetwork/lnd/routing/route"
)

var (
	// ErrUnknownOffer is returned when we receive messages related to
	// an offer that we do not know.
	ErrUnknownOffer = errors.New("unknown offer")

	// ErrInvoiceUnexpected is returned when we receive an invoice for an
	// offer when we are not expecting one.
	ErrInvoiceUnexpected = errors.New("invoice unexpected")

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

	// outboundOffers maps an offer ID to the current state of the offer
	// exchange. This map should *only* be accessed by the handleOffers
	// control loop to ensure consistency.
	outboundOffers map[lntypes.Hash]*activeOfferState

	// paymentResults is a channel used to deliver the results of
	// asynchronous payments made to offer invoices to the coordinator's
	// main loop.
	paymentResults chan *paymentResult

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
		outboundOffers:  make(map[lntypes.Hash]*activeOfferState),
		paymentResults:  make(chan *paymentResult),
		requestShutdown: requestShutdown,
		quit:            make(chan struct{}),
	}
}

// paymentResult summarizes the result of an offer payment.
type paymentResult struct {
	offerID lntypes.Hash
	success bool
}

// newPaymentResult produces a payment result for the offer.
func newPaymentResult(offerID lntypes.Hash, success bool) *paymentResult {
	return &paymentResult{
		offerID: offerID,
		success: success,
	}
}

// OfferPayState represents the various states of an offer exchange when we
// are the paying party.
type OfferPayState int

const (
	// OfferStateInitiated indicates that we have initiated the process of
	// paying an offer.
	OfferStateInitiated OfferPayState = iota

	// OfferStateRequestSent indicates that we have sent an invoice request
	// for the offer.
	OfferStateRequestSent

	// OfferStateInvoiceRecieved indicates that we have received an invoice
	// in response to our request.
	OfferStateInvoiceRecieved

	// OfferStatePaymentDispatched indicates that we have dispatched a
	// payment for the offer.
	OfferStatePaymentDispatched

	// OfferStatePaid indicates that we have paid an offer's invoice.
	OfferStatePaid

	// OfferStateFailed indicates that we failed to pay an offer's invoice.
	OfferStateFailed
)

// activeOfferState represents an offer that is currently active.
type activeOfferState struct {
	// offer references the original offer.
	offer *lnwire.Offer

	// state reflects the current state of the offer.
	state OfferPayState
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
		// Consume the outcomes of payments to offer invoices.
		case result := <-c.paymentResults:
			// It's pretty serious if we're paying offers that we
			// don't know, so we'll exit on an unknown offer.
			offer, ok := c.outboundOffers[result.offerID]
			if !ok {
				return fmt.Errorf("result for unknown offer: "+
					"%v", result.offerID)
			}
			// TODO - make these errors vars for testing?
			if offer.state != OfferStatePaymentDispatched {
				return fmt.Errorf("unexpected offer state: %v "+
					"%v", result.offerID, offer.state)
			}

			newState := OfferStatePaid
			if !result.success {
				newState = OfferStateFailed
			}

			offer.state = newState
			log.Infof("Offer: %v updated to: %v", result.offerID,
				newState)

		case <-c.quit:
			return ErrShuttingDown
		}
	}
}

// handleInvoice handles incoming invoices, checking that we have valid
// in-flight offers associated with them and making payment where required.
func (c *Coordinator) handleInvoice(invoice *lnwire.Invoice) error {
	activeOffer, ok := c.outboundOffers[invoice.OfferID]
	if !ok {
		return fmt.Errorf("%w: offer id: %v received invoice",
			ErrUnknownOffer, invoice.OfferID)
	}

	// Check that the invoice is appropriate for the offer we have on
	// record.
	if err := validateExchange(activeOffer.offer, invoice); err != nil {
		return fmt.Errorf("%w: invoice: %v invalid for offer: %v",
			err, invoice.PaymentHash, invoice.OfferID)
	}

	// Check that we're in the correct state to make a payment.
	if activeOffer.state != OfferStateRequestSent {
		return fmt.Errorf("%w: offer in state: %v",
			ErrInvoiceUnexpected, activeOffer.state)
	}

	// Update our active offer's state.
	activeOffer.state = OfferStateInvoiceRecieved

	dest, err := route.NewVertexFromBytes(
		invoice.NodeID.SerializeCompressed(),
	)
	if err != nil {
		return fmt.Errorf("invalid node id: %w", err)
	}

	// Create a payment request from the invoice we've received.
	req := lndclient.SendPaymentRequest{
		Target:      dest,
		Amount:      invoice.Amount.ToSatoshis(),
		PaymentHash: &invoice.PaymentHash,
		Timeout:     time.Minute * 5,
		// TODO - default to 18 if zero?
		FinalCLTVDelta: uint16(invoice.CLTVExpiry),
	}

	// Update state to indicate that we've dispatched a payment and pay
	// the invoice.
	activeOffer.state = OfferStatePaymentDispatched

	ctx, cancel := context.WithCancel(context.Background())
	payChan, errChan, err := c.lnd.SendPayment(ctx, req)
	if err != nil {
		// cancel our context to prevent a lost cancel linter error (lnd
		// will cancel the payment if it fails, so not strictly
		// required).
		cancel()
		return fmt.Errorf("send payment failed: %w", err)
	}

	c.wg.Add(1)
	go func() {
		// Cancel our lnd context when this loop exits so that the
		// steam with lnd will be canceled if we're shutting down.
		defer func() {
			cancel()
			c.wg.Done()
		}()

		c.monitorPayment(
			invoice.OfferID, invoice.PaymentHash, payChan, errChan,
		)
	}()

	return nil
}

func (c *Coordinator) monitorPayment(offerID, hash lntypes.Hash,
	payChan chan lndclient.PaymentStatus, errChan chan error) {

	for {
		select {
		case status := <-payChan:
			switch status.State {
			case lnrpc.Payment_FAILED:
				log.Infof("Offer: %v, payment: %v failed: %v",
					offerID, hash, status.FailureReason)

				result := newPaymentResult(offerID, false)
				c.deliverPaymentResult(result)

				return

			case lnrpc.Payment_SUCCEEDED:
				log.Infof("Offer: %v, payment: %v succeeded",
					offerID, hash)

				result := newPaymentResult(offerID, true)
				c.deliverPaymentResult(result)

				return

			case lnrpc.Payment_IN_FLIGHT:
				log.Infof("Offer: %v, payment: %v in flight"+
					"%v htlcs", len(status.Htlcs), offerID,
					hash)
			}

		case err := <-errChan:
			// TODO - clean up payment state?
			log.Errorf("offer: %v,payment: %v failed: %v", offerID,
				hash, err)

			return

		// Exit if the coordinator is shutting down.
		case <-c.quit:
			return
		}
	}
}

// deliverPaymentResult delivers the outcome of a payment to our main control
// loop. Delivery is not guaranteed, the result will be dropped is the
// coordinator is shutting down.
func (c *Coordinator) deliverPaymentResult(result *paymentResult) {
	select {
	case c.paymentResults <- result:
	case <-c.quit:
		log.Infof("Offer: %v result not delivered", result.offerID)
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

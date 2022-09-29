package offers

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/carlakc/boltnd/lnwire"
	"github.com/lightninglabs/lndclient"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lntypes"
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

	// ErrPayerKeyMismatch is returned when an invoice request's payer
	// key does not match the invoice.
	ErrPayerKeyMismatch = errors.New("request payer key does not match " +
		"invoice")

	// ErrPayerInfoMismatch is returned when an invoice request's note does
	// not match the invoice.
	ErrPayerInfoMismatch = errors.New("request info does not match " +
		"invoice")

	// ErrPayerNoteMismatch is returned when an invoice request's note
	// does not match the invoice.
	ErrPayerNoteMismatch = errors.New("request note does not match " +
		"invoice")

	// ErrQuantityMismatch is returned when an invoice's quantity does not
	// match the original invoice.
	ErrQuantityMismatch = errors.New("request quantity does not match " +
		"invoice")

	// ErrChainhashMismatch is returned when an offer, invoice request or
	// invoice don't have matching chain hashes.
	ErrChainhashMismatch = errors.New("chainhash does not match")

	// ErrUnknownOffer is returned when an offer is unknown to us.
	ErrUnknownOffer = errors.New("unknown offer")

	// ErrUnexpectedState is returned when an offer does not have the
	// state we expect.
	ErrUnexpectedState = errors.New("unexpected state")
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

	// Signal goroutines to shutdown.
	close(c.quit)

	// Wait for all goroutines to exit.
	c.wg.Wait()

	return nil
}

// handleOffers is the main goroutine that handles offer exchanges.
func (c *Coordinator) handleOffers() error {
	for {
		select {
		// Consume the outcomes of payments to offer invoices.
		case result := <-c.paymentResults:
			// Handle payment of individual offers. We don't error
			// out here because offer-level failures shouldn't shut
			// us down.
			err := c.handlePaymentResult(
				result.offerID, result.success,
			)
			if err != nil {
				log.Errorf("Handle result for offer: %v "+
					"failed: %v", result.offerID, err)
			}

		case <-c.quit:
			return ErrShuttingDown
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

// handlePaymentResult updates the coordinator's internal state when we get
// a payment result for an offer.
func (c *Coordinator) handlePaymentResult(offerID lntypes.Hash,
	success bool) error {

	// It's pretty serious if we're paying offers that we don't know, so
	// we'll exit on an unknown offer.
	offer, ok := c.outboundOffers[offerID]
	if !ok {
		return fmt.Errorf("Offer: %v: %w", offerID, ErrUnknownOffer)
	}

	if offer.state != OfferStatePaymentDispatched {
		return fmt.Errorf("Offer: %v: %w, expected: %v got: %v",
			offerID, ErrUnexpectedState,
			OfferStatePaymentDispatched, offer.state)
	}

	newState := OfferStatePaid
	if !success {
		newState = OfferStateFailed
	}

	offer.state = newState
	log.Infof("Offer: %v updated to: %v", offerID, newState)

	return nil
}

// monitorPayment tracks the progress of an in-flight payment, using the
// deliverResult closure provided to report the final result. The quit channel
// is provided to signal shutdown.
func monitorPayment(offerID, hash lntypes.Hash,
	payChan chan lndclient.PaymentStatus, errChan chan error,
	deliverResult func(*paymentResult), quit chan struct{}) error {

	for {
		select {
		case status := <-payChan:
			switch status.State {
			case lnrpc.Payment_FAILED:
				log.Infof("Offer: %v, payment: %v failed: %v",
					offerID, hash, status.FailureReason)

				result := newPaymentResult(offerID, false)
				deliverResult(result)

				return nil

			case lnrpc.Payment_SUCCEEDED:
				log.Infof("Offer: %v, payment: %v succeeded",
					offerID, hash)

				result := newPaymentResult(offerID, true)
				deliverResult(result)

				return nil

			case lnrpc.Payment_IN_FLIGHT:
				log.Infof("Offer: %v, payment: %v in flight"+
					"%v htlcs", len(status.Htlcs), offerID,
					hash)
			}

		case err := <-errChan:
			log.Errorf("offer: %v,payment: %v failed: %v", offerID,
				hash, err)

			return err

		// Exit if instructed to.
		case <-quit:
			return ErrShuttingDown
		}
	}
}

// validateExchange validates that the messages that form part of a single
// offer exchange are valid. This function assumes that the invoice request
// is valid for the offer, focusing on validation of the invoice against the
// offer and request respectively.
func validateExchange(o *lnwire.Offer, r *lnwire.InvoiceRequest,
	i *lnwire.Invoice) error {

	// We need all chain hashes to be equal, so we just check against the
	// offer for request and invoice. If these values are an empty hash,
	// validation will still pass.
	if !bytes.Equal(o.Chainhash[:], r.Chainhash[:]) {
		return fmt.Errorf("%w: offer: %v, invoice request: %v",
			ErrChainhashMismatch, o.Chainhash, r.Chainhash)
	}

	if !bytes.Equal(o.Chainhash[:], i.Chainhash[:]) {
		return fmt.Errorf("%w: offer: %v, invoice: %v",
			ErrChainhashMismatch, o.Chainhash, i.Chainhash)
	}

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

	if r.PayerKey != i.PayerKey {
		return fmt.Errorf("%w: request: %v, invoice: %v",
			ErrPayerKeyMismatch, r.PayerKey, i.PayerKey)
	}

	if !bytes.Equal(r.PayerInfo, i.PayerInfo) {
		return fmt.Errorf("%w: request: %v, invoice: %v",
			ErrPayerInfoMismatch, r.PayerInfo, i.PayerInfo)
	}

	if r.PayerNote != i.PayerNote {
		return fmt.Errorf("%w: request: %v, invoice: %v",
			ErrPayerNoteMismatch, r.PayerInfo, i.PayerInfo)
	}

	if r.Quantity != i.Quantity {
		return fmt.Errorf("%w: request: %v, invoice: %v",
			ErrQuantityMismatch, r.Quantity, i.Quantity)
	}

	// - MUST reject the invoice if the `offer_id` does not refer an
	// unexpired offer with `send_invoice`

	return nil
}

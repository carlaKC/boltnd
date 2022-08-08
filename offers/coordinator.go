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
		case <-c.quit:
			return ErrShuttingDown
		}
	}
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

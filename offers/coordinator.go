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
	"github.com/carlakc/boltnd/onionmsg"
	"github.com/carlakc/boltnd/routes"
	"github.com/lightninglabs/lndclient"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/lightningnetwork/lnd/routing/route"
)

// defaultCLTVDelta is the default cltv provided by the offers spec.
const defaultCLTVDelta = 18

// defaultPaymentTimeout is the default timeout for lnd payments.
var defaultPaymentTimeout = time.Minute * 5

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

	// onionMessenger provides the external onion messaging functionality
	// required for offers.
	onionMessenger onionmsg.OnionMessenger

	// routeGenerator provides blinded route creation.
	routeGenerator routes.Generator

	// outboundOffers maps an offer ID to the current state of the offer
	// exchange. This map should *only* be accessed by the handleOffers
	// control loop to ensure consistency.
	outboundOffers map[lntypes.Hash]*activeOfferState

	// paymentResults is a channel used to deliver the results of
	// asynchronous payments made to offer invoices to the coordinator's
	// main loop.
	paymentResults chan *paymentResult

	// incomingInvoices is a channel used to deliver incoming invoices
	// delivered over onion messages.
	incomingInvoices chan []byte

	// requestShutdown is called when the messenger experiences an error to
	// signal to calling code that it should gracefully exit.
	requestShutdown func(err error)

	wg   sync.WaitGroup
	quit chan struct{}
}

// NewCoordinator creates a new offer coordinator.
func NewCoordinator(lnd LNDOffers, onionMsgr onionmsg.OnionMessenger,
	routeGenerator routes.Generator,
	requestShutdown func(err error)) *Coordinator {

	return &Coordinator{
		lnd:              lnd,
		onionMessenger:   onionMsgr,
		routeGenerator:   routeGenerator,
		outboundOffers:   make(map[lntypes.Hash]*activeOfferState),
		paymentResults:   make(chan *paymentResult),
		incomingInvoices: make(chan []byte),
		requestShutdown:  requestShutdown,
		quit:             make(chan struct{}),
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
	// offer is the original offer being paid.
	offer *lnwire.Offer

	// invoiceRequest is the request for an invoice that was sent for an
	// active offer.
	invoiceRequest *lnwire.InvoiceRequest

	// state reflects the current state of the offer.
	state OfferPayState
}

// Start runs the coordinator.
func (c *Coordinator) Start() error {
	if !atomic.CompareAndSwapInt32(&c.started, 0, 1) {
		return errors.New("coordinator already started")
	}

	// Register our invoice handler to manage all incoming invoices via
	// onion messages.
	if err := c.onionMessenger.RegisterHandler(
		lnwire.InvoiceNamespaceType,
		c.HandleInvoice,
	); err != nil {
		return fmt.Errorf("could not register invoice handler: %w",
			err)
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

	// De-register our invoice handler.
	err := c.onionMessenger.DeregisterHandler(lnwire.InvoiceNamespaceType)
	if err != nil {
		return fmt.Errorf("deregister invoice hander: %w", err)
	}

	// Wait for all goroutines to exit.
	c.wg.Wait()

	return nil
}

// HandleInvoice delivers an incoming invoice to our main event loop.
func (c *Coordinator) HandleInvoice(_ *lnwire.ReplyPath, _ []byte,
	invoice []byte) error {

	// TODO - does the caller need a way to cancel?
	select {
	case c.incomingInvoices <- invoice:
		return nil

	case <-c.quit:
		return ErrShuttingDown
	}
}

// handleOffers is the main goroutine that handles offer exchanges.
func (c *Coordinator) handleOffers() error {
	for {
		select {
		// Handle incoming invoices.
		case invBytes := <-c.incomingInvoices:
			// Decode the incoming invoice. Do not exit if we fail
			// here, because then we can be shut down by counter
			// parties sending us junk.
			invoice, err := lnwire.DecodeInvoice(invBytes)
			if err != nil {
				log.Errorf("Decode invoice: %v", err)
				continue
			}

			// Check that the invoice is valid.
			err = c.validateReceivedInvoice(invoice)
			if err != nil {
				log.Errorf("Invalid invoice: %v", err)
				continue
			}

			// Create a request to pay this invoice and send it
			// to lnd.
			req, err := createSendPaymentRequest(invoice)
			if err != nil {
				log.Errorf("Invoice payment: %v", err)
				continue
			}

			err = c.sendOfferPayment(invoice.OfferID, *req)
			if err != nil {
				log.Errorf("Send invoice payment: %v", err)
				continue
			}

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

// validateReceivedInvoice performs the various validation checks required
// when we receive an invoice:
// 1. That the invoice itself is valid
// 2. That is is for an offer we want to pay, and we're expecting an incoming
//    invoice for the offer
// 3. That the invoice is valid for the offer / invoice request it is
//    associated with
func (c *Coordinator) validateReceivedInvoice(invoice *lnwire.Invoice) error {
	if err := invoice.Validate(); err != nil {
		return fmt.Errorf("invalid invoice: %w", err)
	}

	activeOffer, ok := c.outboundOffers[invoice.OfferID]
	if !ok {
		return fmt.Errorf("%w: offer id: %v received invoice",
			ErrUnknownOffer, invoice.OfferID)
	}

	// Check that we're in the correct state to make a payment.
	if activeOffer.state != OfferStateRequestSent {
		return fmt.Errorf("Offer: %v: %w, expected: %v got: %v",
			activeOffer.offer.MerkleRoot, ErrUnexpectedState,
			OfferStateRequestSent, activeOffer.state)
	}

	// Check that the invoice is appropriate for the offer we have on
	// record.
	err := validateExchange(
		activeOffer.offer, activeOffer.invoiceRequest, invoice,
	)
	if err != nil {
		return fmt.Errorf("%w: invoice: %v invalid for offer: %v",
			err, invoice.PaymentHash, invoice.OfferID)
	}

	// Once we've validated that this is an invoice that we're expecting
	// and want to pay, we can update our state to reflect that the offer
	// has received an invoice.
	c.outboundOffers[invoice.OfferID].state = OfferStateInvoiceRecieved

	return nil
}

// sendOfferPayment dispatches a payment for an offer via lnd, kicking off a
// goroutine which will monitor and report the outcome of the payment back to
// our main loop. This function assumes that the offer being paid is known to
// the coordinator.
func (c *Coordinator) sendOfferPayment(offerID lntypes.Hash,
	req lndclient.SendPaymentRequest) error {

	// We assume that our offer is known, so we can directly lookup and
	// update our state.
	c.outboundOffers[offerID].state = OfferStatePaymentDispatched

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

		err := monitorPayment(
			offerID, *req.PaymentHash, payChan, errChan,
			c.deliverPaymentResult, c.quit,
		)
		if err != nil && err != ErrShuttingDown {
			log.Errorf("Monitoring payment failed: %v", err)
		}
	}()

	return nil
}

// createSendPaymentRequest creates a request for lnd to pay a bolt 12 invoice.
//
// NB: this function rounds down to satoshis because lnd client's
// SendPaymentRequest currently only includes a satoshi amount field.
func createSendPaymentRequest(invoice *lnwire.Invoice) (
	*lndclient.SendPaymentRequest, error) {

	// Create a payment request from the invoice we've received.
	dest, err := route.NewVertexFromBytes(
		invoice.NodeID.SerializeCompressed(),
	)
	if err != nil {
		return nil, fmt.Errorf("invalid node id: %w", err)
	}

	req := lndclient.SendPaymentRequest{
		Target: dest,
		// TODO - update lndclient to have msat amount!
		Amount:         invoice.Amount.ToSatoshis(),
		PaymentHash:    &invoice.PaymentHash,
		Timeout:        defaultPaymentTimeout,
		FinalCLTVDelta: uint16(invoice.CLTVExpiry),
	}

	// Update to defaults if not specified.
	if req.FinalCLTVDelta == 0 {
		req.FinalCLTVDelta = defaultCLTVDelta
	}

	return &req, nil
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

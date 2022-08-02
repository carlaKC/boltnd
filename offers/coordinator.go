package offers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/carlakc/boltnd/lnwire"
	"github.com/carlakc/boltnd/onionmsg"
	"github.com/lightninglabs/lndclient"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lntypes"
	lndwire "github.com/lightningnetwork/lnd/lnwire"
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

// SendMessage is the function signature used to send messages over an
// abstracted messaging layer.
type SendMessage func(ctx context.Context, peer route.Vertex,
	finalPayloads []*lnwire.FinalHopPayload) error

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

	// outboundOffers maps an offer ID to the current state of the offer
	// exchange. This map should *only* be accessed by the handleOffers
	// control loop to ensure consistency.
	outboundOffers map[lntypes.Hash]*activeOfferState

	// offerRequests is a channel that is used to deliver requests to
	// pay offers to the coordinator's main loop.
	offerRequests chan *offerRequest

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
	requestShutdown func(err error)) *Coordinator {

	return &Coordinator{
		lnd:              lnd,
		onionMessenger:   onionMsgr,
		outboundOffers:   make(map[lntypes.Hash]*activeOfferState),
		offerRequests:    make(chan *offerRequest),
		paymentResults:   make(chan *paymentResult),
		incomingInvoices: make(chan []byte),
		requestShutdown:  requestShutdown,
		quit:             make(chan struct{}),
	}
}

// offerRequest holds request to pay an offer and a response channel to pipe
// results to the caller.
type offerRequest struct {
	// offer is the decoded offer that we wish to pay.
	offer *lnwire.Offer

	// amount is the amount to pay.
	amount lndwire.MilliSatoshi

	// payerNote is an arbitrary note from the sender.
	payerNote string

	// errChan is used to communicate errors to the caller.
	errChan chan error
}

// newOfferRequest creates a new offer request.
func newOfferRequest(o *lnwire.Offer, amount lndwire.MilliSatoshi,
	payerNote string) *offerRequest {

	return &offerRequest{
		offer:     o,
		amount:    amount,
		payerNote: payerNote,

		// Buffer the error channel by 1 so that we are not blocked
		// on the recipient reading results.
		errChan: make(chan error, 1),
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

	// OfferStateMiscFailure indicates that we experienced a failure along
	// the way. TODO - make this better.
	OfferStateMiscFailure
)

// activeOfferState represents an offer that is currently active.
type activeOfferState struct {
	// offer references the original offer.
	offer *lnwire.Offer

	// state reflects the current state of the offer.
	state OfferPayState

	// errChan is a channel that can be used to pass errors to the caller.
	errChan chan<- error
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

	// Signal all goroutines to stop.
	close(c.quit)

	// De-register our invoice handler.
	err := c.onionMessenger.DeregisterHandler(lnwire.InvoiceNamespaceType)
	if err != nil {
		return fmt.Errorf("deregister invoice hander: %w", err)
	}

	// Wait for goroutines to exit.
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

// PayOffer dispatches a request to the offer coordinator to pay an offer.
func (c *Coordinator) PayOffer(offer *lnwire.Offer, amount lndwire.MilliSatoshi,
	payerNote string) <-chan error {

	request := newOfferRequest(offer, amount, payerNote)

	select {
	// Deliver the offer request to the main loop.
	case c.offerRequests <- request:
		log.Debugf("Offer: %v delivered to main loop", offer.MerkleRoot)

	// If we exit before the offer is picked up, immediately return an
	/// error update.
	case <-c.quit:
		// We can send this error response directly into the error
		// channel because we expect it to be buffered.
		request.errChan <- ErrShuttingDown
	}

	return request.errChan
}

// handleOffers is the main goroutine that handles offer exchanges.
func (c *Coordinator) handleOffers() error {
	for {
		select {
		// Handle incoming requests to pay offers
		case request := <-c.offerRequests:
			// TODO - convert to lntypes.Hash
			hash, _ := lntypes.MakeHash(request.offer.MerkleRoot[:])

			_, ok := c.outboundOffers[hash]
			if ok {
				// error out
			}

			// Create an active offer and add it to our set of
			// outbound offers.
			activeOffer := &activeOfferState{
				offer:   request.offer,
				state:   OfferStateInitiated,
				errChan: request.errChan,
			}
			c.outboundOffers[hash] = activeOffer

			// Dispatch a request for an invoice for the offer.
			err := c.dispatchInvoiceRequest(
				request.offer, request.amount,
				request.payerNote,
			)
			if err != nil {
				request.errChan <- err
				continue
			}

			// Once we've successfully dispatched an invoice
			// request, we can update the offer's state to invoice
			// requested.
			activeOffer.state = OfferStateRequestSent

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
			if err := invoice.Validate(); err != nil {
				log.Errorf("Invalid invoice: %v", err)
				continue
			}

			// Handle the incoming invoice, dispatching a payment
			// if required.
			if err := c.handleInvoice(invoice); err != nil {
				log.Errorf("Handle invoice: %v", err)
				continue
			}

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

// dispatchInvoiceRequest creates an invoice request for offer and amount
// provided and dispatches it to the target node.
func (c *Coordinator) dispatchInvoiceRequest(offer *lnwire.Offer,
	amount lndwire.MilliSatoshi, payerNote string) error {

	// TODO - extend lnd interface and derive key / sign.
	payerKey, err := btcec.NewPrivateKey()
	if err != nil {
		return fmt.Errorf("payer key: %w", err)
	}

	request, err := lnwire.NewInvoiceRequest(
		// TODO - set quantity != 1
		offer, amount, 1, payerKey.PubKey(), payerNote,
	)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	digest := request.SignatureDigest()
	sig, err := schnorr.Sign(payerKey, digest[:])
	if err != nil {
		return fmt.Errorf("sign request: %w", err)
	}

	// Copy the signature into our request.
	sigBytes := sig.Serialize()
	copy(request.Signature[:], sigBytes)

	// Finally, encode the invoice request and send it in an onion message.
	requestBytes, err := lnwire.EncodeInvoiceRequest(request)
	if err != nil {
		return fmt.Errorf("invoice request encode: %w", err)
	}

	finalHopPayloads := []*lnwire.FinalHopPayload{
		{
			TLVType: lnwire.InvoiceRequestNamespaceType,
			Value:   requestBytes,
		},
	}

	peer, err := route.NewVertexFromBytes(
		offer.NodeID.SerializeCompressed(),
	)
	if err != nil {
		return fmt.Errorf("offer node ID: %w", err)
	}

	err = c.sendMsg(context.Background(), peer, finalHopPayloads)
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}

	return nil
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

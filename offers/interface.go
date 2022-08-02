package offers

import (
	"context"

	"github.com/carlakc/boltnd/lnwire"
	"github.com/lightninglabs/lndclient"
	"github.com/lightningnetwork/lnd/lntypes"
	lndwire "github.com/lightningnetwork/lnd/lnwire"
)

// LNDOffers is an interface describing the lnd dependencies that the offers
// package requires.
type LNDOffers interface {
	// SendPayment attempts to route a payment to the final destination. The
	// call returns a payment update stream and an error stream.
	SendPayment(ctx context.Context, request lndclient.SendPaymentRequest) (
		chan lndclient.PaymentStatus, chan error, error)

	// TrackPayment picks up a previously started payment and returns a
	// payment update stream and an error stream.
	TrackPayment(ctx context.Context, hash lntypes.Hash) (
		chan lndclient.PaymentStatus, chan error, error)
}

// OfferCoordinator is an interface implemented by components that manage the
// pay lifecycle of an offer.
type OfferCoordinator interface {
	// Start runs the offer coordinator.
	Start() error

	// Stop shuts down the offer coordinator.
	Stop() error

	// PayOffer attempts to make a payment of the amount specified to the
	// offer provided.
	PayOffer(offer *lnwire.Offer, amount lndwire.MilliSatoshi,
		payerNote string) <-chan error
}

// Compile time assertion that coordinator satisfies the OfferCoordinator
// interface.
var _ OfferCoordinator = (*Coordinator)(nil)

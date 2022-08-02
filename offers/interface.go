package offers

import (
	"context"

	"github.com/lightninglabs/lndclient"
	"github.com/lightningnetwork/lnd/lntypes"
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

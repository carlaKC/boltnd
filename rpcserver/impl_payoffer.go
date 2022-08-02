package rpcserver

import (
	"context"

	"github.com/carlakc/boltnd/lnwire"
	"github.com/carlakc/boltnd/offers"
	"github.com/carlakc/boltnd/offersrpc"
	lndwire "github.com/lightningnetwork/lnd/lnwire"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// PayOffer makes a payment to a bolt 12 offer.
func (s *Server) PayOffer(ctx context.Context, req *offersrpc.PayOfferRequest) (
	*offersrpc.PayOfferResponse, error) {

	offer, amount, err := parsePayOfferRequest(req)
	if err != nil {
		return nil, err
	}

	// TODO:
	// - make this a stream immediately
	// - update the way the coordinator delivers updates
	errChan := s.offerCoordinator.PayOffer(offer, amount, req.PayerNote)
	select {
	case err, ok := <-errChan:
		// If our error channel is closed, the offer is complete.
		if !ok {
			return &offersrpc.PayOfferResponse{}, nil
		}

		return nil, err

	// Fail if the client cancels.
	case <-ctx.Done():
		return nil, ctx.Err()

	// Fail if the server is shutting down.
	case <-s.quit:
		return nil, ErrShuttingDown
	}
}

func parsePayOfferRequest(req *offersrpc.PayOfferRequest) (*lnwire.Offer,
	lndwire.MilliSatoshi, error) {

	if req.Offer == "" {
		return nil, 0, status.Error(
			codes.InvalidArgument, "offer string required",
		)
	}

	offer, err := offers.DecodeOfferStr(req.Offer)
	if err != nil {
		return nil, 0, status.Errorf(
			codes.InvalidArgument, "decode offer failed: %v", err,
		)
	}

	amtMsat := lndwire.MilliSatoshi(req.AmountMsat)
	if amtMsat < offer.MinimumAmount {
		return nil, 0, status.Errorf(
			codes.InvalidArgument, "amount: %v must be at "+
				"least: %v", amtMsat, offer.MinimumAmount,
		)
	}

	return offer, amtMsat, nil
}

package rpcserver

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/carlakc/boltnd/offers"
	"github.com/carlakc/boltnd/offersrpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// DecodeOffer decodes and validates the offer string provided.
func (s *Server) DecodeOffer(ctx context.Context,
	req *offersrpc.DecodeOfferRequest) (*offersrpc.DecodeOfferResponse,
	error) {

	log.Debugf("DecodeOffer: %+v", req)

	if err := s.waitForReady(ctx); err != nil {
		return nil, err
	}

	offerStr, err := parseDecodeOfferRequest(req)
	if err != nil {
		return nil, err
	}

	offer, err := offers.DecodeOfferStr(offerStr)
	if err != nil {
		return nil, err
	}

	return composeDecodeOfferResponse(offer)
}

// parseDecodeOfferRequest parses and validates the parameters provided
// by DecodeOfferRequest. All errors returned *must* include a grpc status
// code.
func parseDecodeOfferRequest(req *offersrpc.DecodeOfferRequest) (string,
	error) {

	if req.Offer == "" {
		return "", status.Error(
			codes.InvalidArgument, "offer string required",
		)
	}

	return req.Offer, nil
}

// composeDecodeOfferResponse creates a DecodeOfferResponse from the internal
// offer type.
func composeDecodeOfferResponse(offer *offers.Offer) (
	*offersrpc.DecodeOfferResponse, error) {

	rpcOffer := &offersrpc.Offer{
		MinAmountMsat: uint64(offer.MinimumAmount),
		Description:   offer.Description,
		Issuer:        offer.Issuer,
		MinQuantity:   offer.QuantityMin,
		MaxQuantity:   offer.QuantityMax,
	}

	if offer.Features != nil {
		buf := new(bytes.Buffer)

		if err := offer.Features.Encode(buf); err != nil {
			// We really shouldn't run into issues encoding our
			// feature vector, so we return an internal error if
			// something goes wrong here.
			return nil, status.Error(codes.Internal, fmt.Sprintf(
				"encode features: %v", err,
			))
		}

		rpcOffer.Features = buf.Bytes()
	}

	if !offer.Expiry.IsZero() {
		rpcOffer.ExpiryUnixSeconds = uint64(offer.Expiry.Unix())
	}

	if offer.NodeID != nil {
		rpcOffer.NodeId = hex.EncodeToString(
			schnorr.SerializePubKey(offer.NodeID),
		)
	}

	if offer.Signature != nil {
		rpcOffer.Signature = hex.EncodeToString(offer.Signature[:])
	}

	return &offersrpc.DecodeOfferResponse{
		Offer: rpcOffer,
	}, nil
}

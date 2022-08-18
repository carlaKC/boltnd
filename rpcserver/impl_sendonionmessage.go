package rpcserver

import (
	"context"

	"github.com/carlakc/boltnd/lnwire"
	"github.com/carlakc/boltnd/offersrpc"
	"github.com/lightningnetwork/lnd/routing/route"
	"github.com/lightningnetwork/lnd/tlv"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SendOnionMessage sends an onion message to the peer specified.
func (s *Server) SendOnionMessage(ctx context.Context,
	req *offersrpc.SendOnionMessageRequest) (
	*offersrpc.SendOnionMessageResponse, error) {

	log.Debugf("SendOnionMessage: %+v", req)

	if err := s.waitForReady(ctx); err != nil {
		return nil, err
	}

	pubkey, finalHop, err := parseSendOnionMessageRequest(req)
	if err != nil {
		return nil, err
	}

	err = s.onionMsgr.SendMessage(ctx, pubkey, nil, finalHop)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal, "send message failed: %v", err,
		)
	}

	return &offersrpc.SendOnionMessageResponse{}, nil
}

// parseSendOnionMessageRequest parses and validates the parameters provided
// by SendOnionMessageRequest. All errors returned *must* include a grpc status
// code.
func parseSendOnionMessageRequest(req *offersrpc.SendOnionMessageRequest) (
	route.Vertex, []*lnwire.FinalHopPayload, error) {

	pubkey, err := route.NewVertexFromBytes(req.Pubkey)
	if err != nil {
		return route.Vertex{}, nil, status.Errorf(
			codes.InvalidArgument, "peer pubkey: %v", err,
		)
	}

	// If we have no final payloads, just return nil (this makes it easier
	// for testing than having an empty slice).
	if len(req.FinalPayloads) == 0 {
		return pubkey, nil, nil
	}

	finalHopPayloads := make(
		[]*lnwire.FinalHopPayload, 0, len(req.FinalPayloads),
	)

	for tlvType, payload := range req.FinalPayloads {
		finalPayload := &lnwire.FinalHopPayload{
			TLVType: tlv.Type(tlvType),
			Value:   payload,
		}

		if err := finalPayload.Validate(); err != nil {
			return route.Vertex{}, nil, status.Errorf(
				codes.InvalidArgument, err.Error(),
			)
		}

		finalHopPayloads = append(finalHopPayloads, finalPayload)
	}

	return pubkey, finalHopPayloads, nil
}

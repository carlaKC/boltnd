package rpcserver

import (
	"context"

	"github.com/carlakc/boltnd/offersrpc"
	"github.com/lightningnetwork/lnd/routing/route"
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

	pubkey, err := parseSendOnionMessageRequest(req)
	if err != nil {
		return nil, err
	}

	err = s.onionMsgr.SendMessage(ctx, pubkey)
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
	route.Vertex, error) {

	pubkey, err := route.NewVertexFromBytes(req.Pubkey)
	if err != nil {
		return route.Vertex{}, status.Errorf(
			codes.InvalidArgument, "peer pubkey: %v", err,
		)
	}

	return pubkey, nil
}

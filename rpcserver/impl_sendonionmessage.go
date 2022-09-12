package rpcserver

import (
	"context"
	"errors"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/carlakc/boltnd/lnwire"
	"github.com/carlakc/boltnd/offersrpc"
	"github.com/carlakc/boltnd/onionmsg"
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

	onionReq, err := parseSendOnionMessageRequest(req)
	if err != nil {
		return nil, err
	}

	err = s.onionMsgr.SendMessage(ctx, onionReq)
	switch {
	// If we got a no path error, prompt user to try direct connect if
	// they want to.
	case errors.Is(err, onionmsg.ErrNoPath):
		return nil, status.Errorf(
			codes.NotFound, "could not find path to destination "+
				"try using direct connect to deliver to peer "+
				"(! exposes IP !)",
		)

	// Otherwise fail generically.
	case err != nil:
		return nil, status.Errorf(
			codes.Internal, "send message failed: %v", err,
		)

	default:
		return &offersrpc.SendOnionMessageResponse{}, nil
	}
}

// parseSendOnionMessageRequest parses and validates the parameters provided
// by SendOnionMessageRequest. All errors returned *must* include a grpc status
// code.
func parseSendOnionMessageRequest(req *offersrpc.SendOnionMessageRequest) (
	*onionmsg.SendMessageRequest, error) {

	var (
		pubkeySet  = len(req.Pubkey) != 0
		blindedSet = req.BlindedDestination != nil

		pubkey      *btcec.PublicKey
		blindedDest *lnwire.ReplyPath
		err         error
	)

	switch {
	case pubkeySet && blindedSet:
		return nil, status.Errorf(
			codes.InvalidArgument, "set either pubkey or blinded,"+
				"not both",
		)
	case !(pubkeySet || blindedSet):
		return nil, status.Errorf(
			codes.InvalidArgument, "pubkey or blinded required",
		)

	case pubkeySet:
		pubkey, err = btcec.ParsePubKey(req.Pubkey)
		if err != nil {
			return nil, status.Errorf(
				codes.InvalidArgument, "peer pubkey: %v",
				err.Error(),
			)
		}

	case blindedSet:
		blindedDest, err = parseReplyPath(req.BlindedDestination)
		if err != nil {
			return nil, status.Errorf(
				codes.InvalidArgument, "blinded dest: %v",
				err.Error(),
			)
		}
	}

	replyPath, err := parseReplyPath(req.ReplyPath)
	if err != nil {
		return nil, err
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
			return nil, status.Errorf(
				codes.InvalidArgument, err.Error(),
			)
		}

		finalHopPayloads = append(finalHopPayloads, finalPayload)
	}

	onionReq, err := onionmsg.NewSendMessageRequest(
		pubkey, blindedDest, replyPath, finalHopPayloads,
		req.DirectConnect,
	)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument,
			"onion request: %v", err.Error())
	}

	return onionReq, nil
}

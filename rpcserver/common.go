package rpcserver

import (
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/carlakc/boltnd/lnwire"
	"github.com/carlakc/boltnd/offersrpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// parseReplyPath parses a reply path provided over rpc.
func parseReplyPath(req *offersrpc.BlindedPath) (*lnwire.ReplyPath, error) {
	if req == nil {
		return nil, nil
	}

	intro, err := btcec.ParsePubKey(req.IntroductionNode)
	if err != nil {
		return nil, status.Errorf(
			codes.InvalidArgument, "introduction node: %v",
			err.Error(),
		)
	}

	blinding, err := btcec.ParsePubKey(req.BlindingPoint)
	if err != nil {
		return nil, status.Errorf(
			codes.InvalidArgument, "blinding point: %v",
			err.Error(),
		)
	}

	replyPath := &lnwire.ReplyPath{
		FirstNodeID:   intro,
		BlindingPoint: blinding,
		Hops: make(
			[]*lnwire.BlindedHop, len(req.Hops),
		),
	}

	for i, hop := range req.Hops {
		pubkey, err := btcec.ParsePubKey(hop.BlindedNodeId)
		if err != nil {
			return nil, status.Errorf(
				codes.InvalidArgument, "introduction node: %v",
				err.Error(),
			)
		}

		replyPath.Hops[i] = &lnwire.BlindedHop{
			BlindedNodeID: pubkey,
			EncryptedData: hop.EncrypedData,
		}
	}

	return replyPath, nil
}

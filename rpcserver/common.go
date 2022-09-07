package rpcserver

import (
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/carlakc/boltnd/lnwire"
	"github.com/carlakc/boltnd/offersrpc"
	sphinx "github.com/lightningnetwork/lightning-onion"
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
			EncryptedData: hop.EncryptedData,
		}
	}

	return replyPath, nil
}

// composeReplyPath coverts a reply path to a rpc blinded path.
func composeReplyPath(resp *lnwire.ReplyPath) *offersrpc.BlindedPath {
	if resp == nil {
		return nil
	}

	blindedPath := &offersrpc.BlindedPath{
		IntroductionNode: resp.FirstNodeID.SerializeCompressed(),
		BlindingPoint:    resp.BlindingPoint.SerializeCompressed(),
		Hops:             make([]*offersrpc.BlindedHop, len(resp.Hops)),
	}

	for i, hop := range resp.Hops {
		blindedPath.Hops[i] = &offersrpc.BlindedHop{
			BlindedNodeId: hop.BlindedNodeID.SerializeCompressed(),
			EncryptedData: hop.EncryptedData,
		}
	}

	return blindedPath
}

func composeBlindedRoute(route *sphinx.BlindedPath) *offersrpc.BlindedPath {
	rpcRoute := &offersrpc.BlindedPath{
		IntroductionNode: route.IntroductionPoint.SerializeCompressed(),
	}

	if len(route.EncryptedData) > 0 {
		rpcRoute.IntroductionEncryptedData = route.EncryptedData[0]
	}

	for i := 1; i < len(route.BlindedHops); i++ {
		rpcRoute.Hops[i] = &offersrpc.BlindedHop{
			BlindedNodeId: route.BlindedHops[i].SerializeCompressed(),
			EncryptedData: route.EncryptedData[i],
		}
	}

	return rpcRoute
}

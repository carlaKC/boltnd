package onionmsg

import (
	"bytes"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/carlakc/boltnd/lnwire"
	sphinx "github.com/lightningnetwork/lightning-onion"
)

// encodeBlindedPayload is the function signature used to encode a TLV stream
// of blinded route data for onion messages.
type encodeBlindedPayload func(*btcec.PublicKey) ([]byte, error)

// createPathToBlind takes a set of public keys and creates a set of hops in
// a blinded route. The first node in the route is considered to be the
// introduction node N(0), and all nodes after it are denoted N(1), N(2), etc.
//
// Given a path N(0), N(1), N(2), ... , N(k), the blinded route will have
// the following entries.
// [0] NodePub: N(0)
//     Payload: TLV( next_node_id : N(1) )
// [1] NodePub: N(1)
//     Payload: TLV( next_node_id: N(2) )
// ...
// [k] NodePub: N(k)
//
// An encodePayload function is passed in as a parameter for easy mocking in
// tests.
//
// Note that this function currently sends empty onion messages to peers (no
// TLVs in the final hop).
func createPathToBlind(path []*btcec.PublicKey,
	encodePayload encodeBlindedPayload) ([]*sphinx.BlindedPathHop, error) {

	hopCount := len(path)

	// Create a set of blinded hops for our path.
	hopsToBlind := make([]*sphinx.BlindedPathHop, len(path))

	// Create our first hop, which it the introduction node.
	hopsToBlind[0] = &sphinx.BlindedPathHop{
		NodePub: path[0],
	}

	// Run through all paths and add the cleartext node ID to the
	// previous hop's payload. We need each hop to have the next node's ID
	// in its payload so that it can unblind the route.
	for i := 1; i < hopCount; i++ {
		// Add this node's cleartext pubkey to the previous node's
		// payload.
		var err error
		hopsToBlind[i-1].Payload, err = encodePayload(path[i])
		if err != nil {
			return nil, fmt.Errorf("intermediate node: %v "+
				"encoding failed: %w", i, err)
		}

		// Add our hop to the set of blinded hops.
		hopsToBlind[i] = &sphinx.BlindedPathHop{
			NodePub: path[i],
		}
	}

	return hopsToBlind, nil
}

// blindedToSphinx converts the blinded path provided to a sphinx path that can
// be wrapped up in an onion.
func blindedToSphinx(blindedRoute *sphinx.BlindedPath) (*sphinx.PaymentPath,
	error) {

	var sphinxPath sphinx.PaymentPath

	// We fill our first hop in with the introduction point for our route
	// and its encrypted data. We specifically separate this hop out because
	// we do not want to use the blinded node ID for the first hop.
	/*sphinxPath[0] = sphinx.OnionHop{
		NodePub: *blindedRoute.IntroductionPoint,
		HopPayload: sphinx.HopPayload{
			Type:    sphinx.PayloadTLV,
			Payload: blindedRoute.EncryptedData[0],
		},
	}*/

	// For all remaining hops, we'll fill in the blinded node id and
	// encrypted data.
	for i := 0; i < len(blindedRoute.EncryptedData); i++ {
		sphinxPath[i] = sphinx.OnionHop{
			NodePub: *blindedRoute.BlindedHops[i],
			HopPayload: sphinx.HopPayload{
				Type:    sphinx.PayloadTLV,
				Payload: blindedRoute.EncryptedData[i],
			},
		}
	}

	return &sphinxPath, nil
}

// encodeBlindedData encodes a TLV stream for an intermediate hop in a
// blinded route, including only a next_node_id TLV for onion messaging.
func encodeBlindedData(nextHop *btcec.PublicKey) ([]byte, error) {
	if nextHop == nil {
		return nil, fmt.Errorf("expected non-nil next hop")
	}

	data := &lnwire.BlindedRouteData{
		NextNodeID: nextHop,
	}

	bytes, err := lnwire.EncodeBlindedRouteData(data)
	if err != nil {
		return nil, fmt.Errorf("encode blinded: %w", err)
	}

	return bytes, nil
}

// createOnionMessage creates an onion message, blinding the nodes in the path
// provided and including relevant TLVs for blinded relay of messages.
func createOnionMessage(path []*btcec.PublicKey,
	sessionKey *btcec.PrivateKey) (*lnwire.OnionMessage, error) {

	hopCount := len(path)
	if hopCount < 1 {
		return nil, fmt.Errorf("blinded path must have at least 1 hop")
	}

	// Create a blinded path.
	hops, err := createPathToBlind(path, encodeBlindedData)
	if err != nil {
		return nil, fmt.Errorf("path to blind: %w", err)
	}

	// Create a blinded route from our set of hops, encrypting blobs and
	// blinding node keys as required.
	blindedPath, err := sphinx.BuildBlindedPath(sessionKey, hops)
	if err != nil {
		return nil, fmt.Errorf("blinded path: %w", err)
	}

	sphinxPath, err := blindedToSphinx(blindedPath)
	if err != nil {
		return nil, fmt.Errorf("could not create sphinx path: %w", err)
	}

	// Finally, we want to case this all up in an onion.
	onionPacket, err := sphinx.NewOnionPacket(
		// TODO: check whether we need associated data.
		sphinxPath, sessionKey, nil, sphinx.DeterministicPacketFiller,
	)
	if err != nil {
		return nil, fmt.Errorf("new onion packed failed: %w", err)
	}

	buf := new(bytes.Buffer)
	if err := onionPacket.Encode(buf); err != nil {
		return nil, fmt.Errorf("onion packet encode: %w", err)
	}

	return lnwire.NewOnionMessage(
		blindedPath.BlindingPoint, buf.Bytes(),
	), nil
}

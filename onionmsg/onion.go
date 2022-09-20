package onionmsg

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/carlakc/boltnd/lnwire"
	"github.com/lightninglabs/lndclient"
	sphinx "github.com/lightningnetwork/lightning-onion"
	"github.com/lightningnetwork/lnd/routing/route"
)

var (
	// ErrNoEncryptedData is returned when the encrypted data TLV is not
	// present when it is required.
	ErrNoEncryptedData = errors.New("encrypted data blob required")

	// ErrNoForwardingPayload is returned when no onion message payload
	// is provided to allow forwarding messages.
	ErrNoForwardingPayload = errors.New("no payload provided for " +
		"forwarding")
)

// encodeBlindedPayload is the function signature used to encode a TLV stream
// of blinded route data for onion messages.
type encodeBlindedPayload func(*lnwire.BlindedRouteData) ([]byte, error)

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
// If an introduction node to a blinded path is provided, the path needs to be
// connected to its introduction node and the ephemeral key override included
// in the final payload of the path:
// ...
// [k] NodePub: N(K)
//     Payload: TLV( next_node_id: intro , override: blinding_point )
//
// An encodePayload function is passed in as a parameter for easy mocking in
// tests.
func createPathToBlind(path []*btcec.PublicKey, blindedStart *introductionNode,
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
		// data.
		data := &lnwire.BlindedRouteData{
			NextNodeID: path[i],
		}

		var err error
		hopsToBlind[i-1].Payload, err = encodePayload(data)
		if err != nil {
			return nil, fmt.Errorf("intermediate node: %v "+
				"encoding failed: %w", i, err)
		}

		// Add our hop to the set of blinded hops.
		hopsToBlind[i] = &sphinx.BlindedPathHop{
			NodePub: path[i],
		}
	}

	// If we need to connect this path to a blinded path, we add a payload
	// for the last hop in our path pointing it to the introduction node
	// and providing the ephemeral key to switch out.
	if blindedStart != nil {
		data := &lnwire.BlindedRouteData{
			NextNodeID:           blindedStart.unblindedID,
			NextBlindingOverride: blindedStart.blindingPoint,
		}

		var err error
		hopsToBlind[hopCount-1].Payload, err = encodePayload(data)
		if err != nil {
			return nil, fmt.Errorf("ephemeral switch out node: %v",
				err)
		}
	}

	return hopsToBlind, nil
}

// blindedToSphinx converts the blinded path provided to a sphinx path that can
// be wrapped up in an onion, encoding the TLV payload for each hop along the
// way.
func blindedToSphinx(blindedRoute *sphinx.BlindedPath, extraHops []*blindedHop,
	replyPath *lnwire.ReplyPath, finalPayloads []*lnwire.FinalHopPayload) (
	*sphinx.PaymentPath, error) {

	var sphinxPath sphinx.PaymentPath

	// Fill in the blinded node id and encrypted data for all hops. This
	// requirement differs from blinded hops used for payments, where we
	// don't use the blinded introduction node id. However, since onion
	// messages are fully blinded by default, we use the blinded
	// introduction node id.
	ourHopCount := len(blindedRoute.EncryptedData)
	extraHopCount := len(extraHops)

	for i := 0; i < ourHopCount; i++ {
		// Create an onion message payload with the encrypted data for
		// this hop.
		payload := &lnwire.OnionMessagePayload{
			EncryptedData: blindedRoute.EncryptedData[i],
		}

		// If we're on the final hop and there are no extra hops to add
		// onto our path, include the tlvs intended for the final hop
		// and the reply path (if provided).
		if i == ourHopCount-1 && extraHopCount == 0 {
			payload.FinalHopPayloads = finalPayloads
			payload.ReplyPath = replyPath
		}

		// Encode the tlv stream for inclusion in our message.
		payloadTLVs, err := lnwire.EncodeOnionMessagePayload(payload)
		if err != nil {
			return nil, fmt.Errorf("payload: %v encode: %v", i,
				err)
		}

		sphinxPath[i] = sphinx.OnionHop{
			NodePub: *blindedRoute.BlindedHops[i],
			HopPayload: sphinx.HopPayload{
				Type:    sphinx.PayloadTLV,
				Payload: payloadTLVs,
			},
		}
	}

	// If we don't have any more hops to append to our path, just return
	// it as-is here.
	if extraHopCount == 0 {
		return &sphinxPath, nil
	}

	for i := 0; i < extraHopCount; i++ {
		payload := &lnwire.OnionMessagePayload{
			EncryptedData: extraHops[i].blindedData,
		}

		// If we're on the last hop, add our optional final payload
		// and reply path.
		if i == extraHopCount-1 {
			payload.FinalHopPayloads = finalPayloads
			payload.ReplyPath = replyPath
		}

		payloadTLVs, err := lnwire.EncodeOnionMessagePayload(payload)
		if err != nil {
			return nil, fmt.Errorf("payload: %v encode: %v", i,
				err)
		}

		// We need to offset our index in the sphinx path by the
		// number of hops that we added in the loop above.
		sphinxIndex := i + ourHopCount
		sphinxPath[sphinxIndex] = sphinx.OnionHop{
			NodePub: *extraHops[i].blindedNode,
			HopPayload: sphinx.HopPayload{
				Type:    sphinx.PayloadTLV,
				Payload: payloadTLVs,
			},
		}
	}

	return &sphinxPath, nil
}

// encodeBlindedData encodes a TLV stream for an intermediate hop in a
// blinded route, including only a next_node_id TLV for onion messaging.
func encodeBlindedData(data *lnwire.BlindedRouteData) ([]byte, error) {
	if data.NextNodeID == nil {
		return nil, fmt.Errorf("expected non-nil next hop")
	}

	bytes, err := lnwire.EncodeBlindedRouteData(data)
	if err != nil {
		return nil, fmt.Errorf("encode blinded: %w", err)
	}

	return bytes, nil
}

// createOnionMessage creates an onion message, blinding the nodes in the path
// provided and including relevant TLVs for blinded relay of messages.
func createOnionMessage(sphinxPath *sphinx.PaymentPath,
	sessionKey *btcec.PrivateKey, blindingPoint *btcec.PublicKey) (
	*lnwire.OnionMessage, error) {

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

	return lnwire.NewOnionMessage(blindingPoint, buf.Bytes()), nil
}

// customOnionMessage encodes the onion message provided and wraps it in a
// lnd custom message so that it can be sent to peers via external apis.
func customOnionMessage(peer *btcec.PublicKey,
	msg *lnwire.OnionMessage) (*lndclient.CustomMessage, error) {

	buf := new(bytes.Buffer)
	if err := msg.Encode(buf, 0); err != nil {
		return nil, fmt.Errorf("onion message encode: %w", err)
	}

	return &lndclient.CustomMessage{
		Peer:    route.NewVertex(peer),
		MsgType: lnwire.OnionMessageType,
		Data:    buf.Bytes(),
	}, nil
}

// decryptBlobFunc returns a closure that can be used to decrypt an onion
// message's encrypted data blob and decode it.
func decryptBlobFunc(nodeKey sphinx.SingleKeyECDH) func(*btcec.PublicKey,
	*lnwire.OnionMessagePayload) (*lnwire.BlindedRouteData, error) {

	return func(blindingPoint *btcec.PublicKey,
		payload *lnwire.OnionMessagePayload) (*lnwire.BlindedRouteData,
		error) {

		if payload == nil {
			return nil, ErrNoForwardingPayload
		}

		if len(payload.EncryptedData) == 0 {
			return nil, ErrNoEncryptedData
		}

		decrypted, err := sphinx.DecryptBlindedData(
			nodeKey, blindingPoint, payload.EncryptedData,
		)
		if err != nil {
			return nil, fmt.Errorf("could not decrypt data "+
				"blob: %w", err)
		}

		data, err := lnwire.DecodeBlindedRouteData(decrypted)
		if err != nil {
			return nil, fmt.Errorf("could not decode data "+
				"blob: %w", err)
		}

		return data, nil
	}
}

package routes

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/carlakc/boltnd/lnwire"
	"github.com/lightninglabs/lndclient"
	sphinx "github.com/lightningnetwork/lightning-onion"
	lndwire "github.com/lightningnetwork/lnd/lnwire"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	// ErrNoChannels is returned when we don't have any open channels, so
	// won't be reachable by onion message.
	ErrNoChannels = errors.New("can't create blinded route with no " +
		"channels")

	// ErrNoPeerChannels is returned when a peer does not have any public
	// channels, so it can't be used to relay onion messages.
	ErrNoPeerChannels = errors.New("peer has no channels")

	// ErrNoNodeInfo is returned when we don't have node information
	// available for a peer (graph sync is imperfect).
	ErrNoNodeInfo = errors.New("no node information available")

	// ErrFeatureMismatch is returned when a peer doesn't have the feautres
	// we need for onion relay.
	ErrFeatureMismatch = errors.New("insufficient node features")

	// ErrNoRelayingPeers is returned when we have no peers that are
	// eligible for inclusion in a route with the feature set we require.
	ErrNoRelayingPeers = errors.New("no relaying peers")

	// ErrNoPath is returned when a request for a blinded route doesn't
	// have sufficient hops.
	ErrNoPath = errors.New("at least one hop required in route request")

	// ErrSessionKeyRequired is returned when a session key is missing from
	// a routes request.
	ErrSessionKeyRequired = errors.New("session key required")

	// ErrBlindingKeyRequired is returned when a blinding key is missing
	// from a routes request.
	ErrBlindingKeyRequired = errors.New("blinding key required")

	// ErrNoIntroductionNode is returned when our introduction node is not
	// the final hop in a path provided for a send to a blinded route.
	ErrNoIntroductionNode = errors.New("introduction node should be " +
		"final hop when sending to a blinded path")
)

// BlindedRouteGenerator produces blinded routes.
type BlindedRouteGenerator struct {
	// lnd provides access to our lnd node.
	lnd Lnd

	// pubkey is our node's public key.
	pubkey *btcec.PublicKey
}

// Compile time check that blinded path generator implements the generator
// interface.
var _ Generator = (*BlindedRouteGenerator)(nil)

// NewBlindedRouteGenerator creates a blinded route generator.
func NewBlindedRouteGenerator(lnd Lnd,
	pubkey *btcec.PublicKey) *BlindedRouteGenerator {

	return &BlindedRouteGenerator{
		lnd:    lnd,
		pubkey: pubkey,
	}
}

// ReplyPath produces a blinded route to our node with the set of features
// requested.
func (b *BlindedRouteGenerator) ReplyPath(ctx context.Context,
	features []lndwire.FeatureBit) (*sphinx.BlindedPath, error) {

	canRelay := createRelayCheck(features)
	peers, err := getRelayingPeers(ctx, b.lnd, canRelay)
	if err != nil {
		return nil, fmt.Errorf("get relaying peers: %w", err)
	}

	hops, err := buildBlindedRoute(peers, b.pubkey)
	if err != nil {
		return nil, fmt.Errorf("blinded route: %w", err)
	}

	sessionKey, err := btcec.NewPrivateKey()
	if err != nil {
		return nil, fmt.Errorf("session key: %w", err)
	}

	route, err := sphinx.BuildBlindedPath(sessionKey, hops)
	if err != nil {
		return nil, fmt.Errorf("sphinx blinded route: %w", err)
	}

	return route, nil
}

// buildBlindedRoute produces a blinded route from a set of peers that can relay
// onion messages to our node.
//
// TODO - this has terrible privacy, fill in more nodes (or dummies) between
// us and the intro node.
func buildBlindedRoute(relayingPeers []*lndclient.NodeInfo,
	ourPubkey *btcec.PublicKey) ([]*sphinx.BlindedPathHop, error) {

	if len(relayingPeers) == 0 {
		return nil, ErrNoRelayingPeers
	}

	var mostPeers *lndclient.NodeInfo
	for _, peer := range relayingPeers {
		if mostPeers == nil {
			mostPeers = peer
		}

		if len(peer.Channels) > len(mostPeers.Channels) {
			mostPeers = peer
		}
	}

	introNode, err := btcec.ParsePubKey(mostPeers.PubKey[:])
	if err != nil {
		return nil, fmt.Errorf("intro pubkey: %w", err)
	}

	introPayload := &lnwire.BlindedRouteData{
		NextNodeID: ourPubkey,
	}

	introPayloadBytes, err := lnwire.EncodeBlindedRouteData(introPayload)
	if err != nil {
		return nil, fmt.Errorf("intro payload: %w", err)
	}

	return []*sphinx.BlindedPathHop{
		{
			NodePub: introNode,
			Payload: introPayloadBytes,
		},
		{
			NodePub: ourPubkey,
			Payload: nil,
		},
	}, nil
}

// canRelayFunc is the function signature of closures used to check whether a
// peer can relay onion messages.
type canRelayFunc func(*lndclient.NodeInfo) error

// getRelayingPeers returns a list of peers that would be suitable for relaying
// onion messages:
// 1. We have a channel with the peer: assuming that onion messages will be
//    predominantly relayed on channel-lines.
// 2. The channel is active: an active channel indicates that the peer is
//    online and will likely be able to relay messages.
// 3. The node satisfies the canRelay closure passed in (provided as a param
//    for easy testing).
func getRelayingPeers(ctx context.Context, lnd Lnd,
	canRelay canRelayFunc) ([]*lndclient.NodeInfo, error) {

	// List all channels (private and inactive) so that we can provide
	// better error messages.
	channels, err := lnd.ListChannels(ctx, true, false)
	if err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}

	// Assuming that onion messages will only be relayed along
	// channel-lines, we fail if we have no channels.
	if len(channels) == 0 {
		return nil, ErrNoChannels
	}

	var activePeers []*lndclient.NodeInfo
	for _, channel := range channels {
		// Lookup the peer in our graph. Skip over any peers that
		// aren't found (gossip sync is imperfect), but fail if we
		// error out otherwise.
		nodeInfo, err := lnd.GetNodeInfo(ctx, channel.PubKeyBytes, true)
		if err != nil {
			// If we don't have an error code, or we have one that
			// isn't "NotFound" then an unexpected error has
			// occurred.
			status, ok := status.FromError(err)
			if !ok || status.Code() != codes.NotFound {
				return nil, fmt.Errorf("get node: %w", err)
			}

			log.Debugf("Node: %x not found in graph",
				channel.PubKeyBytes)

			continue
		}

		if err := canRelay(nodeInfo); err != nil {
			log.Debugf("Node: %x can't relay onion messages: %v",
				channel.PubKeyBytes, err)

			continue
		}

		activePeers = append(activePeers, nodeInfo)
	}

	return activePeers, nil
}

// createRelayCheck returns a function that can be used to check a node's
// channels and features to determine whether we can use it to relay onions.
func createRelayCheck(features []lndwire.FeatureBit) canRelayFunc {
	return func(nodeInfo *lndclient.NodeInfo) error {
		// If the node has no public channels, it likely won't be
		// reachable.
		if len(nodeInfo.Channels) == 0 {
			return ErrNoPeerChannels
		}

		// If we don't have node information, we don't have this node's
		// announcement.
		if nodeInfo.Node == nil {
			return ErrNoNodeInfo
		}

		featureVec := lndwire.NewRawFeatureVector(nodeInfo.Features...)
		for _, feature := range features {
			// We don't need to check our optional features.
			if !feature.IsRequired() {
				continue
			}

			if !featureVec.IsSet(feature) {
				return fmt.Errorf("%w: %v required not not set",
					ErrFeatureMismatch, feature)
			}
		}

		return nil
	}
}

// BlindedRouteRequest contains a request to produce a blinded route.
type BlindedRouteRequest struct {
	// SessionKey is the ephemeral key used for the blinded route's onion.
	sessionKey *btcec.PrivateKey

	// BlindingKey is the private key to be used for blinding hops in the
	// route.
	blindingKey *btcec.PrivateKey

	// Hops is the set of un-blinded hops in the route. Note that if a
	// blinded destination is provided, we expect the last hop in this
	// route to be the introduction node.
	hops []*btcec.PublicKey

	// ReplyPath is an optional reply path to include to allow recipients
	// to respond to our message.
	replyPath *lnwire.ReplyPath

	// blindedDestination is an optional reply path that we want to send
	// this message to.
	blindedDestination *lnwire.ReplyPath

	// FinalPayloads contains any payloads intended for the last hop in
	// the route.
	finalPayloads []*lnwire.FinalHopPayload

	// blindPath blinds the set of hops provided.
	blindPath func(*btcec.PrivateKey, []*sphinx.BlindedPathHop) (
		*sphinx.BlindedPath, error)

	// encodeBlindedData encodes data for blinded route blobs.
	encodeBlindedData func(*lnwire.BlindedRouteData) ([]byte, error)

	// directToBlinded returns a response for the edge case where we are
	// directly connected to the introduction node in our blinded
	// destination.
	directToBlinded func(*BlindedRouteRequest) (*BlindedRouteResponse,
		error)
}

// validate performs sanity checks on a request.
func (r *BlindedRouteRequest) validate() error {
	if len(r.hops) == 0 {
		return ErrNoPath
	}

	if r.sessionKey == nil {
		return ErrSessionKeyRequired
	}

	if r.blindingKey == nil {
		return ErrBlindingKeyRequired
	}

	// If we don't have a blinded destination, we don't need to perform
	// any further validation.
	if r.blindedDestination == nil {
		return nil
	}

	// If we have a blinded destination included, we expect the last hop
	// in our un-blinded hops to be the introduction node because we are
	// connecting our un-blinded route to the blinded route.
	lastHop := r.hops[len(r.hops)-1]
	introNode := r.blindedDestination.FirstNodeID

	if !bytes.Equal(
		lastHop.SerializeCompressed(),
		introNode.SerializeCompressed(),
	) {
		return fmt.Errorf("%w: expected: %x, got: %x",
			ErrNoIntroductionNode,
			introNode.SerializeCompressed(),
			lastHop.SerializeCompressed())
	}

	return nil
}

// NewBlindedRouteRequest produces a request to create a blinded path.
func NewBlindedRouteRequest(sessionKey, blindingKey *btcec.PrivateKey,
	hops []*btcec.PublicKey, replyPath, blindedDest *lnwire.ReplyPath,
	finalPayloads []*lnwire.FinalHopPayload) *BlindedRouteRequest {

	return &BlindedRouteRequest{
		sessionKey:         sessionKey,
		blindingKey:        blindingKey,
		hops:               hops,
		replyPath:          replyPath,
		blindedDestination: blindedDest,
		finalPayloads:      finalPayloads,
		// Fill in functions that we need for non-test path building.
		blindPath:         sphinx.BuildBlindedPath,
		encodeBlindedData: encodeBlindedData,
		directToBlinded:   directToBlinded,
	}
}

// blindedStart returns information about a request's introduction node, if it
// has one.
func (r *BlindedRouteRequest) blindedStart() *blindedStart {
	if r.blindedDestination == nil {
		return nil
	}

	return &blindedStart{
		unblindedID:   r.blindedDestination.FirstNodeID,
		blindingPoint: r.blindedDestination.BlindingPoint,
	}
}

// blindedHops returns any additional blinded hops included in the request that
// need to be added to our path.
func (r *BlindedRouteRequest) blindedHops() []*lnwire.BlindedHop {
	if r.blindedDestination == nil {
		return nil
	}

	return r.blindedDestination.Hops
}

// BlindedRouteRequest contains the output of a request for a blinded route.
type BlindedRouteResponse struct {
	// OnionMessage is the onion message to be sent on the wire.
	OnionMessage *lnwire.OnionMessage

	// FirstNode is the unblinded public key of the node that the onion
	// message should be sent to.
	FirstNode *btcec.PublicKey
}

// CreateBlindedRoute creates a blinded route from the request provided.
func CreateBlindedRoute(req *BlindedRouteRequest) (*BlindedRouteResponse,
	error) {

	if err := req.validate(); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// Save the unblinded pubkey of the first node we need to connect to.
	// We save this value so that we can tell the caller who to dispatch
	// the message to.
	firstNode := req.hops[0]

	// We don't actually want to blind the introduction node (because we
	// already have its blinded pubkey + encrypted data blob from the reply
	// path). Here, we sanity check that the introduction node is indeed the
	// last node in the set of hops and then trim it from the route.
	if req.blindedDestination != nil {
		// Trim our introduction node off of the set of hops to be
		// blinded (we've already checked in validation that this is
		// indeed the final hop).
		hopCount := len(req.hops)
		req.hops = req.hops[:hopCount-1]

		// Once we've trimmed our introduction node, we may have no
		// hops left if we were directly connected to the introduction
		// node (since our path would have just been a single hop to
		// that node). In this edge case, we can just send our onion
		// message to the blinded path provided, since we're not
		// pre-pending any other hops to it.
		if len(req.hops) == 0 {
			return req.directToBlinded(req)
		}
	}

	// Create a set of hops and corresponding blobs to be encrypted which
	// form the route for our blinded path.
	hops, err := createPathToBlind(
		req.hops, req.blindedStart(), req.encodeBlindedData,
	)
	if err != nil {
		return nil, fmt.Errorf("path to blind: %w", err)
	}

	// Create a blinded route from our set of hops, encrypting blobs and
	// blinding node keys as required.
	blindedPath, err := req.blindPath(req.blindingKey, hops)
	if err != nil {
		return nil, fmt.Errorf("blinded path: %w", err)
	}

	// Convert that blinded path to a sphinx path, adding in our reply
	// path and final payloads if required.
	sphinxPath, err := blindedToSphinx(
		blindedPath, req.blindedHops(), req.replyPath, req.finalPayloads,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create sphinx path: %w", err)
	}

	// Combine our onion hops with the reply path and payloads for the
	// recipient to create an onion message.
	onionMsg, err := createOnionMessage(
		sphinxPath, req.sessionKey, req.blindingKey.PubKey(),
	)
	if err != nil {
		return nil, fmt.Errorf("could not create onion message: %w",
			err)
	}

	return &BlindedRouteResponse{
		OnionMessage: onionMsg,
		FirstNode:    firstNode,
	}, nil
}

// encodeBlindedPayload is the function signature used to encode a TLV stream
// of blinded route data for onion messages.
type encodeBlindedPayload func(*lnwire.BlindedRouteData) ([]byte, error)

type blindedStart struct {
	unblindedID   *btcec.PublicKey
	blindingPoint *btcec.PublicKey
}

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
//
// Note that this function currently sends empty onion messages to peers (no
// TLVs in the final hop).
func createPathToBlind(path []*btcec.PublicKey, blindedStart *blindedStart,
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
func blindedToSphinx(blindedRoute *sphinx.BlindedPath,
	extraHops []*lnwire.BlindedHop, replyPath *lnwire.ReplyPath,
	finalPayloads []*lnwire.FinalHopPayload) (
	*sphinx.PaymentPath, error) {

	var (
		sphinxPath sphinx.PaymentPath

		ourHopCount   = len(blindedRoute.EncryptedData)
		extraHopCount = len(extraHops)
	)

	// Fill in the blinded node id and encrypted data for all hops. This
	// requirement differs from blinded hops used for payments, where we
	// don't use the blinded introduction node id. However, since onion
	// messages are fully blinded by default, we use the blinded
	// introduction node id.
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
		hop, err := createSphinxHop(
			*blindedRoute.BlindedHops[i], payload,
		)
		if err != nil {
			return nil, fmt.Errorf("sphinx hop %v: %w", i, err)
		}
		sphinxPath[i] = *hop
	}

	// If we don't have any more hops to append to our path, just return
	// it as-is here.
	if extraHopCount == 0 {
		return &sphinxPath, nil
	}

	for i := 0; i < extraHopCount; i++ {
		payload := &lnwire.OnionMessagePayload{
			EncryptedData: extraHops[i].EncryptedData,
		}

		// If we're on the last hop, add our optional final payload
		// and reply path.
		if i == extraHopCount-1 {
			payload.FinalHopPayloads = finalPayloads
			payload.ReplyPath = replyPath
		}

		hop, err := createSphinxHop(
			*extraHops[i].BlindedNodeID, payload,
		)
		if err != nil {
			return nil, fmt.Errorf("sphinx hop %v: %w", i, err)
		}

		// We need to offset our index in the sphinx path by the
		// number of hops that we added in the loop above.
		sphinxIndex := i + ourHopCount
		sphinxPath[sphinxIndex] = *hop
	}

	return &sphinxPath, nil
}

// createSphinxHop encodes an onion message payload and produces a sphinx
// onion hop for it.
func createSphinxHop(nodeID btcec.PublicKey,
	payload *lnwire.OnionMessagePayload) (*sphinx.OnionHop, error) {

	payloadTLVs, err := lnwire.EncodeOnionMessagePayload(payload)
	if err != nil {
		return nil, fmt.Errorf("payload: encode: %v", err)
	}

	return &sphinx.OnionHop{
		NodePub: nodeID,
		HopPayload: sphinx.HopPayload{
			Type:    sphinx.PayloadTLV,
			Payload: payloadTLVs,
		},
	}, nil
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

// createOnionMessage creates an onion message from the sphinx path provided.
func createOnionMessage(sphinxPath *sphinx.PaymentPath,
	sessionKey *btcec.PrivateKey,
	blindingPoint *btcec.PublicKey) (*lnwire.OnionMessage, error) {

	// Create an onion packet with no associated data (not required by the
	// spec).
	onionPacket, err := sphinx.NewOnionPacket(
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
		blindingPoint, buf.Bytes(),
	), nil
}

// directToBlinded returns a route response when we are just sending directly
// to the blinded destination we have been provided. This covers the edge case
// where we happen to be connected to the introduction node selected by the
// recipient, and we don't need to append any of our hops onto the blinded
// route.
func directToBlinded(req *BlindedRouteRequest) (*BlindedRouteResponse, error) {
	var sphinxPath sphinx.PaymentPath

	for i, hop := range req.blindedDestination.Hops {
		sphinxHop, err := createSphinxHop(
			*hop.BlindedNodeID,
			&lnwire.OnionMessagePayload{
				EncryptedData: hop.EncryptedData,
			},
		)
		if err != nil {
			return nil, fmt.Errorf("sphinx hop "+
				"%v: %w", i, err)
		}

		sphinxPath[i] = *sphinxHop
	}

	// Note: we still use our session key for the onion
	// packet, but provide the blinded reply path's point.
	onionMsg, err := createOnionMessage(
		&sphinxPath, req.sessionKey,
		req.blindedDestination.BlindingPoint,
	)
	if err != nil {
		return nil, fmt.Errorf("onion message to "+
			"blinded path: %w", err)
	}

	return &BlindedRouteResponse{
		OnionMessage: onionMsg,
		FirstNode:    req.blindedDestination.FirstNodeID,
	}, nil
}

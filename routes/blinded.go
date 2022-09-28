package routes

import (
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

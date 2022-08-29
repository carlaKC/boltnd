package routes

import (
	"context"
	"errors"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/lightninglabs/lndclient"
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
)

// BlindedRouteGenerator produces blinded routes.
type BlindedRouteGenerator struct {
	// lnd provides access to our lnd node.
	lnd Lnd

	// pubkey is our node's public key.
	pubkey *btcec.PublicKey
}

// NewBlindedRouteGenerator creates a blinded route generator.
func NewBlindedRouteGenerator(lnd Lnd,
	pubkey *btcec.PublicKey) *BlindedRouteGenerator {

	return &BlindedRouteGenerator{
		lnd:    lnd,
		pubkey: pubkey,
	}
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

			// Log that we're not found.
			continue
		}

		if err := canRelay(nodeInfo); err != nil {
			// log that we can't relay
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

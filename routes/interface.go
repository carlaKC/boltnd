package routes

import (
	"context"

	"github.com/lightninglabs/lndclient"
	sphinx "github.com/lightningnetwork/lightning-onion"
	lndwire "github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/routing/route"
)

// Lnd describes the lnd dependencies required to find a route.
type Lnd interface {
	// ListChannels lists our current set of channels.
	ListChannels(ctx context.Context, activeOnly,
		publicOnly bool) ([]lndclient.ChannelInfo, error)

	// GetNodeInfo looks up a node in the public ln graph.
	GetNodeInfo(ctx context.Context, pubkey route.Vertex,
		includeChannels bool) (*lndclient.NodeInfo, error)
}

// Generator is an interface implemented by blinded route producers.
type Generator interface {
	// BlindedRoute produces a blinded route to our node with the set of
	// features requested.
	BlindedRoute(ctx context.Context,
		features []lndwire.FeatureBit) (*sphinx.BlindedPath, error)
}

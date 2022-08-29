package routes

import (
	"context"

	"github.com/lightninglabs/lndclient"
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

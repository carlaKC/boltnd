package onionmsg

import (
	"context"

	"github.com/lightninglabs/lndclient"
	"github.com/lightningnetwork/lnd/routing/route"
)

// LndOnionMsg is an interface describing the lnd dependencies that the onionmsg
// package required.
type LndOnionMsg interface {
	// SendCustomMessage sends a custom message to a peer.
	SendCustomMessage(ctx context.Context, msg lndclient.CustomMessage) error

	// GetNodeInfo looks up a node in the public ln graph.
	GetNodeInfo(ctx context.Context, pubkey route.Vertex,
		includeChannels bool) (*lndclient.NodeInfo, error)

	// ListPeers returns lnd's current set of peers.
	ListPeers(ctx context.Context) ([]lndclient.Peer, error)

	// Connect makes a connection to the peer provided.
	Connect(ctx context.Context, peer route.Vertex, host string,
		permanent bool) error
}

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

// OnionMessenger is an interface implemented by objects that can send and
// receive onion messages.
type OnionMessenger interface {
	// Start the onion messenger.
	Start() error

	// Stop the onion messenger, blocking until all goroutines exit.
	Stop() error

	// SendMessage sends an onion message to the peer specified.
	SendMessage(ctx context.Context, peer route.Vertex) error
}

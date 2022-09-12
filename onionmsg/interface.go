package onionmsg

import (
	"context"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/lightninglabs/lndclient"
	"github.com/lightningnetwork/lnd/keychain"
	"github.com/lightningnetwork/lnd/routing/route"
	"github.com/lightningnetwork/lnd/tlv"
)

// LndOnionMsg is an interface describing the lnd dependencies that the onionmsg
// package required.
type LndOnionMsg interface {
	// SendCustomMessage sends a custom message to a peer.
	SendCustomMessage(ctx context.Context, msg lndclient.CustomMessage) error

	// SubscribeCustomMessages subscribes to custom messages received by
	// lnd.
	SubscribeCustomMessages(ctx context.Context) (
		<-chan lndclient.CustomMessage, <-chan error, error)

	// GetNodeInfo looks up a node in the public ln graph.
	GetNodeInfo(ctx context.Context, pubkey route.Vertex,
		includeChannels bool) (*lndclient.NodeInfo, error)

	// ListPeers returns lnd's current set of peers.
	ListPeers(ctx context.Context) ([]lndclient.Peer, error)

	// Connect makes a connection to the peer provided.
	Connect(ctx context.Context, peer route.Vertex, host string,
		permanent bool) error

	// GetInfo returns information about the lnd node.
	GetInfo(ctx context.Context) (*lndclient.Info, error)

	// QueryRoutes queries lnd for a route to a destination peer.
	QueryRoutes(ctx context.Context, req lndclient.QueryRoutesRequest) (
		*lndclient.QueryRoutesResponse, error)
}

// LndOnionSigner is an interface describing the lnd dependencies required for
// the onion messenger's cryptographic operations.
type LndOnionSigner interface {
	// DeriveSharedKey returns a shared secret key by performing
	// Diffie-Hellman key derivation between the ephemeral public key and
	// the key specified by the key locator (or the node's identity private
	// key if no key locator is specified)
	DeriveSharedKey(ctx context.Context, ephemeralPubKey *btcec.PublicKey,
		keyLocator *keychain.KeyLocator) ([32]byte, error)
}

// OnionMessenger is an interface implemented by objects that can send and
// receive onion messages.
type OnionMessenger interface {
	// Start the onion messenger.
	Start() error

	// Stop the onion messenger, blocking until all goroutines exit.
	Stop() error

	// SendMessage sends an onion message to the peer specified. A set of
	// optional TLVs for the target peer can be included in final payloads.
	SendMessage(ctx context.Context, req *SendMessageRequest) error

	// RegisterHandler adds a handler onion message payloads delivered to
	// our node for the tlv type provided.
	// Note: this function will fail if the messenger has not been started.
	RegisterHandler(tlvType tlv.Type, handler OnionMessageHandler) error

	// DeregisterHandler removes a handler for onion message payloads for
	// the tlv type provided.
	// Note: this function will fail if the messenger has not been started.
	DeregisterHandler(tlvType tlv.Type) error
}

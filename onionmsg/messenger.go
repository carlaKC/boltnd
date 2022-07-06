package onionmsg

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/carlakc/boltnd/lnwire"
	"github.com/lightninglabs/lndclient"
	"github.com/lightningnetwork/lnd/routing/route"
)

const (
	lookupPeerBackoffDefault  = time.Second * 1
	lookupPeerAttemptsDefault = 5
)

var (
	// ErrNoAddresses is returned when we can't find a node's address in the
	// public graph.
	ErrNoAddresses = errors.New("no advertised addresses")

	// ErrNoConnection is returned if we don't successfully connect to our
	// peer within our set number of retries.
	ErrNoConnection = errors.New("peer not connected within wait period")
)

// Messenger houses the functionality to send and receive onion messages.
type Messenger struct {
	// lnd provides the lnd apis required for onion messaging.
	lnd LndOnionMsg

	// lookupPeerBackoff is the amount of time that we back off for when
	// waiting to connect to a peer.
	lookupPeerBackoff time.Duration

	// lookupPeerAttempts is the number of times we try to lookup our peer
	// once connected.
	lookupPeerAttempts int
}

// NewOnionMessenger creates a new onion messenger.
func NewOnionMessenger(lnd LndOnionMsg) *Messenger {
	return &Messenger{
		lnd:                lnd,
		lookupPeerBackoff:  lookupPeerBackoffDefault,
		lookupPeerAttempts: lookupPeerAttemptsDefault,
	}
}

// Compile time check that Messenger satisfies the OnionMessenger interface.
var _ OnionMessenger = (*Messenger)(nil)

// SendMessage sends an onion message to the peer provided. If we are not
// currently connected to a peer, the messenger will directly connect to it
// and send the message.
//
// Note that this API doesn't currently include a payload for the message, it
// just sends an empty onion message.
func (m *Messenger) SendMessage(ctx context.Context, peer route.Vertex) error {
	msg, err := customOnionMessage(peer)
	if err != nil {
		return fmt.Errorf("could not create message: %w", err)
	}

	// Check whether we're connected to the next peer, making an ad-hoc
	// connection if we're not.
	if err := m.lookupAndConnect(ctx, peer); err != nil {
		return fmt.Errorf("lookup and connect: %w", err)
	}

	return m.lnd.SendCustomMessage(ctx, *msg)
}

// lookupAndConnect checks whether we have a connection with a peer, and  looks
// it up in the graph and makes a connection if we're not already connected.
func (m *Messenger) lookupAndConnect(ctx context.Context,
	peer route.Vertex) error {

	// If we're already peered with the node, exit early.
	isPeer, err := m.findPeer(ctx, peer)
	if err != nil {
		return fmt.Errorf("find peer: %w", err)
	}

	if isPeer {
		return nil
	}

	info, err := m.lnd.GetNodeInfo(ctx, peer, false)
	if err != nil {
		return fmt.Errorf("could not lookup node: %w", err)
	}

	if len(info.Addresses) == 0 {
		return fmt.Errorf("%w: %v", ErrNoAddresses, peer)
	}

	// Make a permanent connection to the peer so that they don't get
	// pruned because we don't have a channel with them.
	err = m.lnd.Connect(ctx, peer, info.Addresses[0], true)
	if err != nil {
		return fmt.Errorf("could not connect to peer: %w", err)
	}

	// It takes some time for our peer to connect, so we
	for i := 0; i < m.lookupPeerAttempts; i++ {
		isPeer, err := m.findPeer(ctx, peer)
		if err != nil {
			return fmt.Errorf("find peer: %v", err)
		}

		if isPeer {
			return nil
		}

		// If we're not yet peered with the node, back off (or exit
		// if ctx is canceled).
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-time.After(m.lookupPeerBackoff):
			continue
		}
	}

	return ErrNoConnection
}

// findPeer looks for a peer's pubkey in our list of online peers.
func (m *Messenger) findPeer(ctx context.Context, peer route.Vertex) (bool,
	error) {

	peers, err := m.lnd.ListPeers(ctx)
	if err != nil {
		return false, fmt.Errorf("list peers: %w", err)
	}

	// If we're already peers, we can just return early.
	for _, currentPeer := range peers {
		if currentPeer.Pubkey == peer {
			return true, nil
		}
	}

	return false, nil
}

// customOnionMessage creates an onion message to our peer and wraps it in
// a custom lnd message.
func customOnionMessage(peer route.Vertex) (*lndclient.CustomMessage, error) {
	pubkey, err := btcec.ParsePubKey(peer[:])
	if err != nil {
		return nil, fmt.Errorf("invalid pubkey: %w", err)
	}

	path := []*btcec.PublicKey{
		pubkey,
	}

	sessionKey, err := btcec.NewPrivateKey()
	if err != nil {
		return nil, fmt.Errorf("could not get session key: %w", err)
	}

	// Create and encode an onion message.
	msg, err := createOnionMessage(path, sessionKey)
	if err != nil {
		return nil, fmt.Errorf("onion message creation failed: %v", err)
	}

	buf := new(bytes.Buffer)
	if err := msg.Encode(buf, 0); err != nil {
		return nil, fmt.Errorf("onion message encode: %w", err)
	}

	return &lndclient.CustomMessage{
		Peer:    peer,
		MsgType: lnwire.OnionMessageType,
		Data:    buf.Bytes(),
	}, nil
}

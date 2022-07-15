package onionmsg

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/carlakc/boltnd/lnwire"
	"github.com/lightninglabs/lndclient"
	sphinx "github.com/lightningnetwork/lightning-onion"
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

	// ErrNoForwarding is returned if we receive an onion message intended
	// to be forwarded, but do not support forwarding.
	ErrNoForwarding = errors.New("received onion message for forwarding, " +
		"not supported")

	// ErrBadMessage is returned when we can't process an onion message.
	ErrBadMessage = errors.New("onion message processing failed")

	// ErrBadOnionMsg is returned when we receive a bad onion message.
	ErrBadOnionMsg = errors.New("invalid onion message")

	// ErrBadOnionBlob is returned when we receive a bad onion blob within
	// our onion message.
	ErrBadOnionBlob = errors.New("invalid onion blob")

	// ErrShuttingDown is returned when the messenger exits.
	ErrShuttingDown = errors.New("messenger shutting down")

	// ErrLNDShutdown is returned when lnd shuts down one of our streams.
	ErrLNDShutdown = errors.New("lnd shutting down")
)

// Messenger houses the functionality to send and receive onion messages.
type Messenger struct {
	started int32 // to be used atomically
	stopped int32 // to be used atomically

	// lnd provides the lnd apis required for onion messaging.
	lnd LndOnionMsg

	// router provides onion routing capabilities for the messenger.
	router *sphinx.Router

	// lookupPeerBackoff is the amount of time that we back off for when
	// waiting to connect to a peer.
	lookupPeerBackoff time.Duration

	// lookupPeerAttempts is the number of times we try to lookup our peer
	// once connected.
	lookupPeerAttempts int

	// requestShutdown is called when the messenger experiences an error to
	// signal to calling code that it should gracefully exit.
	requestShutdown func(err error)

	wg   sync.WaitGroup
	quit chan struct{}
}

// NewOnionMessenger creates a new onion messenger.
func NewOnionMessenger(params *chaincfg.Params, lnd LndOnionMsg,
	nodeKeyECDH sphinx.SingleKeyECDH, shutdown func(error)) *Messenger {

	return &Messenger{
		lnd: lnd,
		router: sphinx.NewRouter(
			nodeKeyECDH, params, sphinx.NewMemoryReplayLog(),
		),
		lookupPeerBackoff:  lookupPeerBackoffDefault,
		lookupPeerAttempts: lookupPeerAttemptsDefault,
		requestShutdown:    shutdown,
		quit:               make(chan struct{}),
	}
}

// Start the messenger, running all goroutines required.
func (m *Messenger) Start() error {
	if !atomic.CompareAndSwapInt32(&m.started, 0, 1) {
		return errors.New("messenger already started")
	}

	log.Info("Starting onion messenger")
	if err := m.router.Start(); err != nil {
		return fmt.Errorf("could not start router: %w", err)
	}

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()

		err := m.receiveOnionMessages(context.Background())
		if err != nil && err != ErrShuttingDown {
			m.requestShutdown(err)
		}
	}()

	return nil
}

// Stop shuts down the messenger and waits for all goroutines to exit.
func (m *Messenger) Stop() error {
	if !atomic.CompareAndSwapInt32(&m.stopped, 0, 1) {
		return fmt.Errorf("messenger already stopped")
	}

	log.Info("Stopping onion messenger")
	defer log.Info("Onion messenger stopped")

	// Signal our goroutines to quit and wait for them to exit.
	close(m.quit)
	m.wg.Wait()

	// Shutdown our onion router. We do this after shutting down goroutines
	// so that any errors due to a stopped router don't error-out before we
	// can cleanly shut down.
	m.router.Stop()

	return nil
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

// receiveOnionMessages consumes onion messages from lnd's custom message
// stream and handles them.
func (m *Messenger) receiveOnionMessages(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	msgChan, errChan, err := m.lnd.SubscribeCustomMessages(ctx)
	if err != nil {
		return err
	}

	for {
		select {
		case msg, ok := <-msgChan:
			// If our message channel has been closed, the stream
			// has exited.
			if !ok {
				return fmt.Errorf("%w: messages", ErrLNDShutdown)
			}

			// Skip over all non-onion messages.
			if msg.MsgType != lnwire.OnionMessageType {
				continue
			}

			// Just log failures for individual onion messages,
			// since we don't want one malformed message to send
			// us down.
			err := handleOnionMessage(m.processOnion, msg)
			if err == nil {
				continue
			}

			// Try to unwrap our error to match it against our
			// various typed errors. If the error is not wrapped,
			// Unwrap will return nil, in which case we match
			// against the original error.
			upwrappedErr := errors.Unwrap(err)
			if upwrappedErr == nil {
				upwrappedErr = err
			}

			// Handle the non-nil error accordingly, we've already
			// managed the nil case above.
			switch upwrappedErr {
			// Log that we're dropping the message if it's supposed
			// to be forwarded (not supported at present).
			case ErrNoForwarding:
				log.Infof("Received onion message with more "+
					"hops from: %v forwarding not "+
					"supported, dropping message", msg.Peer)

			// Don't error out on invalid messages (it allows peers
			// to send us junk to shut us down), just log.
			// TODO: possibly penalize bad messages in future?
			case ErrBadMessage, ErrBadOnionMsg, ErrBadOnionBlob:
				log.Errorf("Processing failed for onion "+
					"packet from: %v: %v", msg.Peer, err)

			// Log any other errors, since a single bad message
			// should not shut us down.
			default:
				log.Errorf("Onion message from: %v failed: %v",
					msg.Peer, err)
			}

		case err, ok := <-errChan:
			// If our error channel has been closed, the stream
			// has exited.
			if !ok {
				return fmt.Errorf("%w: message errors",
					ErrLNDShutdown)
			}

			return fmt.Errorf("message subscription failed: %w",
				err)

		case <-m.quit:
			return ErrShuttingDown
		}
	}
}

// processOnion uses the messenger's router to process onion messages.
func (m *Messenger) processOnion(onionPkt *sphinx.OnionPacket,
	blindingPoint *btcec.PublicKey) (*sphinx.ProcessedPacket, error) {

	return m.router.ProcessOnionPacket(onionPkt, nil, 0, blindingPoint)
}

// processOnion is the function signature used to process onion packets.
type processOnion func(onionPkt *sphinx.OnionPacket,
	blindingPoint *btcec.PublicKey) (*sphinx.ProcessedPacket, error)

// handleOnionMessage extracts onion messages from custom messages received from
// lnd. A process onion closure is passed in for easy testing.
func handleOnionMessage(processOnion processOnion,
	msg lndclient.CustomMessage) error {

	log.Infof("Received onion message from peer: %v", msg.Peer)

	onionMsg := lnwire.OnionMessage{}
	if err := onionMsg.Decode(bytes.NewBuffer(msg.Data), 0); err != nil {
		return fmt.Errorf("%w: %v", ErrBadMessage, err)
	}

	// The onion blob portion of our message holds the actual onion.
	onionPktBytes := bytes.NewBuffer(onionMsg.OnionBlob)

	onionPkt := &sphinx.OnionPacket{}
	if err := onionPkt.Decode(onionPktBytes); err != nil {
		return fmt.Errorf("%w:%v", ErrBadOnionBlob, err)
	}

	processedPacket, err := processOnion(onionPkt, onionMsg.BlindingPoint)
	if err != nil {
		return fmt.Errorf("%w: could not process onion packet: %v",
			ErrBadOnionBlob, err)
	}

	switch processedPacket.Action {
	// If we're the exit node, this onion message was intended for us.
	case sphinx.ExitNode:
		log.Infof("Onion message from: %v is for us!", msg.Peer)

		// TODO: handle processed packet's payload.
		return nil

	// We don't support forwarding at present, so we fail if an onion with
	// more hops is received.
	case sphinx.MoreHops:
		return ErrNoForwarding

	// If we encounter a sphinx failure, just log the error and ignore the
	// packet.
	case sphinx.Failure:
		return ErrBadMessage
	}

	return nil
}

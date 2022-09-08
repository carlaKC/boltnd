package itest

import (
	"context"
	"sync"
	"testing"

	"github.com/carlakc/boltnd/offersrpc"
	"github.com/lightningnetwork/lnd/lntest"
	"github.com/stretchr/testify/require"
)

// OnionMsgForwardTestCase tests forwarding of onion messages.
func OnionMsgForwardTestCase(t *testing.T, net *lntest.NetworkHarness) {
	offersTest := setupForBolt12(t, net)
	defer offersTest.cleanup()

	// Spin up a third node immediately because we will need a three-hop
	// network for this test.
	carol := net.NewNode(t, "carol", []string{onionMsgProtocolOverride})
	carolB12, cleanup := bolt12Client(t, carol)
	defer cleanup()

	// Connect nodes before channel opening so that they can share gossip.
	net.ConnectNodesPerm(t, net.Alice, net.Bob)
	net.ConnectNodesPerm(t, net.Bob, carol)

	// Open channels: Alice --- Bob --- Carol and wait for each node to
	// sync the network graph.
	openChannelAndAnnounce(t, net, net.Alice, net.Bob, carol)
	openChannelAndAnnounce(t, net, net.Bob, carol, net.Alice)

	var (
		ctxb = context.Background()
		wg   sync.WaitGroup
	)

	// Create a context with no timeout that will cancel at the end of our
	// test and wait for any goroutines that have been spun up.
	ctxc, cancel := context.WithCancel(ctxb)
	defer func() {
		cancel()
		wg.Wait()
	}()

	// Setup a subscription to a specific TLV payload.
	var tlvType uint64 = 101
	subReq := &offersrpc.SubscribeOnionPayloadRequest{
		TlvType: tlvType,
	}

	// We expect failed subscriptions to exit quickly, so we use a timeout
	// so that our receives won't block indefinitely.
	client, err := carolB12.SubscribeOnionPayload(
		ctxc, subReq,
	)
	require.NoError(t, err)

	// Setup closures to receive from Carol's subscription.
	var (
		errChan = make(chan error, 1)
		msgChan = make(
			chan *offersrpc.SubscribeOnionPayloadResponse, 1,
		)
	)
	consumeMessage := consumeOnionMessage(&wg, msgChan, errChan)
	receiveMessage := readOnionMessage(msgChan, errChan)

	// Now send an onion message from Alice to Carol without using direct
	// connect. This should prompt Alice to send a multi-hop onion message,
	// which is forwarded by Bob and received by Carol.
	tlvPayload := []byte{1, 2, 3}

	ctxt, cancel := context.WithTimeout(ctxc, defaultTimeout)
	req := &offersrpc.SendOnionMessageRequest{
		Pubkey: carol.PubKey[:],
		FinalPayloads: map[uint64][]byte{
			tlvType: tlvPayload,
		},
	}

	_, err = offersTest.aliceOffers.SendOnionMessage(ctxt, req)
	require.NoError(t, err, "alice -> carol message")
	cancel()

	// Read the message from our subscription and assert that we have the
	// payload we expect.
	consumeMessage(client)
	onionMsg, err := receiveMessage()
	require.NoError(t, err)
	require.Equal(t, tlvPayload, onionMsg.Value)
}

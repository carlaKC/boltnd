package itest

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/carlakc/boltnd/lnwire"
	"github.com/carlakc/boltnd/offersrpc"
	"github.com/carlakc/boltnd/testutils"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lntest"
	"github.com/stretchr/testify/require"
)

// OnionMessageTestCase tests the exchange of onion messages.
func OnionMessageTestCase(t *testing.T, net *lntest.NetworkHarness) {
	offersTest := setupForBolt12(t, net)
	defer offersTest.cleanup()

	var (
		ctxb = context.Background()
		wg   sync.WaitGroup
	)

	// Start with Alice and Bob connected. Alice won't be able to look up
	// Bob in the graph because he doesn't have any channels (and she has
	// no peers for gossip), so we first test the easy case where they're
	// already connected.
	net.ConnectNodes(t, net.Alice, net.Bob)

	// Create a context with no timeout that will cancel at the end of our
	// test and wait for any goroutines that have been spun up.
	ctxc, cancel := context.WithCancel(ctxb)
	defer func() {
		cancel()
		wg.Wait()
	}()

	// Subscribe to custom messages that bob receives.
	bobMsg, err := net.Bob.LightningClient.SubscribeCustomMessages(
		ctxc, &lnrpc.SubscribeCustomMessagesRequest{},
	)
	require.NoError(t, err, "bob subscribe")

	// Send an onion message from alice to bob, using our default timeout
	// to ensure that sending does not hang.
	ctxt, cancel := context.WithTimeout(ctxb, defaultTimeout)
	req := &offersrpc.SendOnionMessageRequest{
		Pubkey:        net.Bob.PubKey[:],
		DirectConnect: true,
	}
	_, err = offersTest.aliceOffers.SendOnionMessage(ctxt, req)
	require.NoError(t, err, "send onion message")
	cancel()

	// We don't want our test to block if we don't receive, so we buffer
	// channels and spin up a goroutine to wait for Bob's message
	// subscription.
	var (
		errChan = make(chan error, 1)
		msgChan = make(chan *lnrpc.CustomMessage, 1)
	)

	// Setup a closure that can be used to receive messages async.
	receiveMessage := func() {
		wg.Add(1)
		go func() {
			defer wg.Done()

			msg, err := bobMsg.Recv()
			if err != nil {
				errChan <- err
				return
			}

			msgChan <- msg
		}()
	}

	// Setup a closure that will consume our received message or fail if
	// nothing is received by a timeout.
	readMessage := func() {
		select {
		// If we receive a message as expected, assert that it is of
		// the correct type.
		case msg := <-msgChan:
			require.Equal(t, uint32(lnwire.OnionMessageType),
				msg.Type)

		// If we received an error, something went wrong.
		case err := <-errChan:
			t.Fatalf("message not received: %v", err)

		// In the case of a timeout, let our test exit. This will
		// cancel the receive goroutine (through context cancelation)
		// and wait for it to exit.
		case <-time.After(defaultTimeout):
			t.Fatal("message not received within timeout")
		}
	}

	// Listen for a message and wait to receive it.
	receiveMessage()
	readMessage()

	// Now, we will spin up a new node, carol to test sending messages to
	// peers that we are not currently connected to.
	carol := net.NewNode(t, "carol", []string{onionMsgProtocolOverride})

	// Connect Alice and Carol so that Carol can sync the graph from Alice.
	net.ConnectNodesPerm(t, net.Alice, carol)

	// We're going to open a channel between Alice and Bob, so that they
	// become part of the public graph.
	chanReq := lntest.OpenChannelParams{
		Amt: 500_0000,
	}

	chanUpdates, err := net.OpenChannel(net.Alice, net.Bob, chanReq)
	require.NoError(t, err, "open channel")

	// Mine 6 blocks so that our channel will confirm.
	_, err = net.Miner.Client.Generate(6)
	require.NoError(t, err, "mine blocks")

	channelID, err := net.WaitForChannelOpen(chanUpdates)
	require.NoError(t, err, "alice chan open")

	// Wait for all nodes to see the channel between Alice and Bob.
	require.NoError(
		t, net.Alice.WaitForNetworkChannelOpen(channelID), "alice wait",
	)
	require.NoError(
		t, net.Bob.WaitForNetworkChannelOpen(channelID), "bob wait",
	)
	require.NoError(
		t, carol.WaitForNetworkChannelOpen(channelID), "carol wait",
	)

	// We now have the following setup:
	//  Alice --- (channel) ---- Bob
	//    |
	// p2p conn
	//    |
	// Carol
	//
	// Carol should be able to send an onion message to Bob by looking
	// him up in the graph and sending to his public address.
	carolB12, cleanup := bolt12Client(t, carol)
	defer cleanup()

	// Send an onion message from Carol to Bob, this time including a reply
	// path to add coverage there.
	var (
		pubkeys = testutils.GetPubkeys(t, 3)
		pubkey0 = pubkeys[0].SerializeCompressed()
		pubkey1 = pubkeys[1].SerializeCompressed()
		pubkey2 = pubkeys[2].SerializeCompressed()
	)
	ctxt, cancel = context.WithTimeout(ctxb, defaultTimeout)
	req = &offersrpc.SendOnionMessageRequest{
		Pubkey: net.Bob.PubKey[:],
		ReplyPath: &offersrpc.BlindedPath{
			IntroductionNode:          pubkey0,
			IntroductionEncryptedData: []byte{1, 2, 3},
			BlindingPoint:             pubkey1,
			Hops: []*offersrpc.BlindedHop{
				{
					BlindedNodeId: pubkey2,
					EncrypedData:  []byte{3, 2, 1},
				},
			},
		},
		DirectConnect: true,
	}
	_, err = carolB12.SendOnionMessage(ctxt, req)
	require.NoError(t, err, "carol message")
	cancel()

	// Listen for a message from Carol -> Bob and wait to receive it.
	receiveMessage()
	readMessage()

	// Now that Alice has a channel open with Bob, she should be able to
	// send an onion message to him without using "direct connect".
	req.DirectConnect = false

	ctxt, cancel = context.WithTimeout(ctxb, defaultTimeout)
	_, err = offersTest.aliceOffers.SendOnionMessage(ctxt, req)
	require.NoError(t, err, "alice -> bob no direct connect")
	cancel()

	receiveMessage()
	readMessage()
}

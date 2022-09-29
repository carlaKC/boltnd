package itest

import (
	"context"
	"sync"
	"testing"

	"github.com/carlakc/boltnd/offersrpc"
	"github.com/lightningnetwork/lnd/lntest"
	"github.com/stretchr/testify/require"
)

// ReplyMessageTestCase tests sending of onion messages to reply paths.
func ReplyMessageTestCase(t *testing.T, net *lntest.NetworkHarness) {
	offersTest := setupForBolt12(t, net)
	defer offersTest.cleanup()

	ctxb := context.Background()

	// Setup our network with the following topology:
	// Alice -- Bob -- Carol -- Dave
	carol := net.NewNode(t, "carol", []string{onionMsgProtocolOverride})
	dave := net.NewNode(t, "dave", []string{onionMsgProtocolOverride})

	// We'll also need a bolt 12 client for dave, because he's going to be
	// receiving our onion messages.
	daveB12, cleanup := bolt12Client(t, dave)
	defer cleanup()

	// First we make p2p connections so that all the nodes can gossip
	// channel information with each other, then we setup the channels
	// themselves.
	net.ConnectNodesPerm(t, net.Alice, net.Bob)
	net.ConnectNodesPerm(t, net.Bob, carol)
	net.ConnectNodesPerm(t, carol, dave)

	// Alice -> Bob
	openChannelAndAnnounce(t, net, net.Alice, net.Bob, carol, dave)

	// Bob -> Carol
	openChannelAndAnnounce(t, net, net.Bob, carol, net.Alice, dave)

	// Carol -> Dave
	fundNode(ctxb, t, net, carol)
	openChannelAndAnnounce(t, net, carol, dave, net.Alice, net.Bob)

	// Create a reply path to Dave's node.
	ctxt, cancel := context.WithTimeout(ctxb, defaultTimeout)
	replyPath, err := daveB12.GenerateBlindedRoute(
		ctxt, &offersrpc.GenerateBlindedRouteRequest{},
	)
	cancel()
	require.NoError(t, err, "reply path")

	// We expect a simple reply path: Carol (intro) --> Dave because our
	// selection is very simple (read: stupid + bad for privacy). Sanity
	// assert that Carol is the intro node.
	require.Equal(t, replyPath.Route.IntroductionNode, carol.PubKey[:],
		"expected carol introduction node")

	// Now subscribe to onion payloads received by dave. We don't add a
	// timeout on this subscription, but rather just cancel it at the end
	// of the test.
	ctxc, cancelSub := context.WithCancel(ctxb)
	defer cancelSub()

	subReq := &offersrpc.SubscribeOnionPayloadRequest{
		TlvType: 101,
	}
	client, err := daveB12.SubscribeOnionPayload(ctxc, subReq)
	require.NoError(t, err, "subscription")

	var (
		errChan = make(chan error, 1)
		msgChan = make(
			chan *offersrpc.SubscribeOnionPayloadResponse, 1,
		)

		wg sync.WaitGroup
	)
	defer wg.Wait()

	// Setup a closure that can be used to consume messages async and one
	// that will read our received messages.
	consumeMessage := consumeOnionMessage(&wg, msgChan, errChan)
	receiveMessage := readOnionMessage(msgChan, errChan)

	// Send an onion message from Alice to Dave's reply path.
	ctxt, cancel = context.WithTimeout(ctxb, defaultTimeout)
	data := []byte{9, 8, 7}
	req := &offersrpc.SendOnionMessageRequest{
		BlindedDestination: replyPath.Route,
		FinalPayloads: map[uint64][]byte{
			subReq.TlvType: data,
		},
	}

	_, err = offersTest.aliceOffers.SendOnionMessage(ctxt, req)
	require.NoError(t, err)
	cancel()

	// Read and receive the message from Dave's subscription and assert
	// that we get the payload we expect.
	consumeMessage(client)
	msg, err := receiveMessage()
	require.NoError(t, err, "receive message 1")
	require.Equal(t, data, msg.Value)

	// Next, test the edge case where we are directly connected to the
	// introduction node of a reply path. We do this by sending from
	// Bob -> Carol (intro) -> Dave along the reply path we generated
	// above. We've already sanity checked that Carol is indeed the
	// introduction node above.
	newData := []byte{1, 2, 3}
	req.FinalPayloads[subReq.TlvType] = newData

	ctxt, cancel = context.WithTimeout(ctxb, defaultTimeout)
	_, err = offersTest.bobOffers.SendOnionMessage(ctxt, req)
	require.NoError(t, err)
	cancel()

	// Wait to receive the next message and assert that it has our new data.
	consumeMessage(client)
	msg, err = receiveMessage()
	require.NoError(t, err, "receive message 2")
	require.Equal(t, newData, msg.Value)
}

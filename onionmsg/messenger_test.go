package onionmsg

import (
	"context"
	"errors"
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/carlakc/boltnd/testutils"
	"github.com/lightninglabs/lndclient"
	sphinx "github.com/lightningnetwork/lightning-onion"
	"github.com/lightningnetwork/lnd/routing/route"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type sendMessageTest struct {
	name string

	// peer is the peer to send our message to.
	peer route.Vertex

	// peerLookups is the number of times that we lookup our peer after
	// connecting.
	peerLookups int

	// expectedErr is the error we expect.
	expectedErr error

	// setMock primes our lnd mock for the specific test case.
	setMock func(*mock.Mock)
}

// TestSendMessage tests sending of onion messages using lnd's custom message
// api.
func TestSendMessage(t *testing.T) {
	pubkeys := testutils.GetPubkeys(t, 1)

	pubkey, err := route.NewVertexFromBytes(
		pubkeys[0].SerializeCompressed(),
	)
	require.NoError(t, err, "pubkey")

	var (
		peerList = []lndclient.Peer{
			{
				Pubkey: pubkey,
			},
		}
		nodeAddr = "host:port"

		privateNodeInfo = &lndclient.NodeInfo{
			Node: &lndclient.Node{},
		}

		nodeInfo = &lndclient.NodeInfo{
			Node: &lndclient.Node{
				Addresses: []string{
					nodeAddr,
				},
			},
		}

		listPeersErr = errors.New("listpeers failed")
		getNodeErr   = errors.New("get node failed")
		connectErr   = errors.New("connect failed")
	)

	tests := []sendMessageTest{
		{
			name:        "success - peer already connected",
			peer:        pubkey,
			peerLookups: 5,
			expectedErr: nil,
			setMock: func(m *mock.Mock) {
				// We are already connected to the peer.
				testutils.MockListPeers(m, peerList, nil)

				// Send the message to the peer.
				testutils.MockSendAnyCustomMessage(m, nil)
			},
		},
		{
			name:        "failure - list peers fails",
			peer:        pubkey,
			peerLookups: 5,
			expectedErr: listPeersErr,
			setMock: func(m *mock.Mock) {
				testutils.MockListPeers(m, nil, listPeersErr)
			},
		},
		{
			name:        "failure - peer not found in graph",
			peer:        pubkey,
			peerLookups: 5,
			expectedErr: getNodeErr,
			setMock: func(m *mock.Mock) {
				// We have no peers at present.
				testutils.MockListPeers(m, nil, nil)

				// Fail because we can't find the peer in the
				// graph.
				testutils.MockGetNodeInfo(
					m, pubkey, false, nil, getNodeErr,
				)
			},
		},
		{
			name:        "failure - peer has no addresses",
			peer:        pubkey,
			peerLookups: 5,
			expectedErr: ErrNoAddresses,
			setMock: func(m *mock.Mock) {
				// We have no peers at present.
				testutils.MockListPeers(m, nil, nil)

				// Peer lookup succeeds, but there are no
				// addresses listed.
				testutils.MockGetNodeInfo(
					m, pubkey, false, privateNodeInfo, nil,
				)
			},
		},

		{
			name:        "failure - could not connect to peer",
			peer:        pubkey,
			expectedErr: connectErr,
			setMock: func(m *mock.Mock) {
				// We have no peers at present.
				testutils.MockListPeers(m, nil, nil)

				// Find the peer in the graph.
				testutils.MockGetNodeInfo(
					m, pubkey, false, nodeInfo, nil,
				)

				// Try to connect to the address provided, fail.
				testutils.MockConnect(
					m, pubkey, nodeAddr, true, connectErr,
				)
			},
		},
		{
			name:        "success - peer immediately found",
			peer:        pubkey,
			peerLookups: 5,
			expectedErr: nil,
			setMock: func(m *mock.Mock) {
				// We have no peers at present.
				testutils.MockListPeers(m, nil, nil)

				// Find the peer in the graph.
				testutils.MockGetNodeInfo(
					m, pubkey, false, nodeInfo, nil,
				)

				// Succeed in connecting to the address
				// provided.
				testutils.MockConnect(
					m, pubkey, nodeAddr, true, nil,
				)

				// After connecting, immediately return the
				// target peer from listpeers.
				testutils.MockListPeers(m, peerList, nil)

				// Send the message to the peer.
				testutils.MockSendAnyCustomMessage(m, nil)
			},
		},
		{
			name:        "success - peer found after retry",
			peer:        pubkey,
			peerLookups: 5,
			expectedErr: nil,
			setMock: func(m *mock.Mock) {
				// We have no peers at present.
				testutils.MockListPeers(m, nil, nil)

				// Find the peer in the graph.
				testutils.MockGetNodeInfo(
					m, pubkey, false, nodeInfo, nil,
				)

				// Succeed in connecting to the address
				// provided.
				testutils.MockConnect(
					m, pubkey, nodeAddr, true, nil,
				)

				// In our first peer lookups, don't return the
				// peer (mocking the time connection / handshake
				// takes.
				testutils.MockListPeers(m, nil, nil)
				testutils.MockListPeers(m, nil, nil)

				// On our third attempt, we're connected to the
				// peer.
				testutils.MockListPeers(m, peerList, nil)

				// Send the message to the peer.
				testutils.MockSendAnyCustomMessage(m, nil)
			},
		},
		{
			name:        "failure - peer not found after retry",
			peer:        pubkey,
			peerLookups: 2,
			expectedErr: ErrNoConnection,
			setMock: func(m *mock.Mock) {
				// We have no peers at present.
				testutils.MockListPeers(m, nil, nil)

				// Find the peer in the graph.
				testutils.MockGetNodeInfo(
					m, pubkey, false, nodeInfo, nil,
				)

				// Succeed in connecting to the address
				// provided.
				testutils.MockConnect(
					m, pubkey, nodeAddr, true, nil,
				)

				// The peer does not show up in our peer list
				// after 2 calls.
				testutils.MockListPeers(m, nil, nil)
				testutils.MockListPeers(m, nil, nil)
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			testSendMessage(t, testCase)
		})
	}
}

func testSendMessage(t *testing.T, testCase sendMessageTest) {
	// Create a mock and prime it for the calls we expect in this test.
	lnd := testutils.NewMockLnd()
	defer lnd.Mock.AssertExpectations(t)

	testCase.setMock(lnd.Mock)

	privkeys := testutils.GetPrivkeys(t, 1)
	nodeKey := privkeys[0]

	// Create a simple SingleKeyECDH impl here for testing.
	nodeKeyECDH := &sphinx.PrivKeyECDH{
		PrivKey: nodeKey,
	}

	// We don't expect the messenger's shutdown function to be used, so
	// we can provide nil (knowing that our tests will panic if it's used).
	messenger := NewOnionMessenger(
		&chaincfg.RegressionNetParams, lnd, nodeKeyECDH, nil,
	)

	// Overwrite our peer lookup defaults so that we don't have sleeps in
	// our tests.
	messenger.lookupPeerAttempts = testCase.peerLookups
	messenger.lookupPeerBackoff = 0

	ctxb := context.Background()

	err := messenger.SendMessage(ctxb, testCase.peer)

	// All of our errors are wrapped, so we can just check err.Is the
	// error we expect (also works for nil).
	require.True(t, errors.Is(err, testCase.expectedErr))
}

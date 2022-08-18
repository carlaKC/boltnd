package onionmsg

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/carlakc/boltnd/lnwire"
	"github.com/carlakc/boltnd/testutils"
	"github.com/lightninglabs/lndclient"
	sphinx "github.com/lightningnetwork/lightning-onion"
	"github.com/lightningnetwork/lnd/routing/route"
	"github.com/lightningnetwork/lnd/tlv"
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

	err := messenger.SendMessage(ctxb, testCase.peer, nil, nil)

	// All of our errors are wrapped, so we can just check err.Is the
	// error we expect (also works for nil).
	require.True(t, errors.Is(err, testCase.expectedErr))
}

// handleOnionMesageMock is a mock that handled all mocked calls for testing
// onion messaging.
type handleOnionMesageMock struct {
	*mock.Mock
}

func (h *handleOnionMesageMock) DecodePayload(o []byte) (
	*lnwire.OnionMessagePayload, error) {

	args := h.Mock.MethodCalled("DecodePayload", o)

	return args.Get(0).(*lnwire.OnionMessagePayload), args.Error(1)
}

// mockPayloadDecode primes the mock to handle a call to decode payload and
// return the payload and error provided. Note that this call does not assert
// that this function is called with a specific value.
func mockPayloadDecode(m *mock.Mock, payload *lnwire.OnionMessagePayload,
	err error) {

	m.On(
		"DecodePayload", mock.Anything,
	).Once().Return(
		payload, err,
	)
}

// OnionMessageHandler mocks a call to handle an onion message.
func (h *handleOnionMesageMock) OnionMessageHandler(path *lnwire.ReplyPath,
	encrypted []byte, payload []byte) error {

	args := h.Mock.MethodCalled(
		"OnionMessageHandler", path, encrypted, payload,
	)

	return args.Error(0)
}

// mockMessageHandled primes the mock to handle a call to an onion message
// handler with the payload provided. The mock will return the error supplied.
func mockMessageHandled(m *mock.Mock, path *lnwire.ReplyPath, data,
	payload []byte, err error) {

	m.On(
		"OnionMessageHandler", path, data, payload,
	).Once().Return(
		err,
	)
}

// TestHandleOnionMessage tests different handling cases for onion messages.
func TestHandleOnionMessage(t *testing.T) {
	pubkeys := testutils.GetPubkeys(t, 3)
	nodeKey, err := route.NewVertexFromBytes(
		pubkeys[0].SerializeCompressed(),
	)
	require.NoError(t, err, "pubkey")

	// Create a single valid message that we can use across test cases.
	msg, err := customOnionMessage(nodeKey, nil, nil)
	require.NoError(t, err, "create msg")

	mockErr := errors.New("mock err")

	// Setup some values to use for our mocked payload decoding.
	replyPath := &lnwire.ReplyPath{
		FirstNodeID:   pubkeys[0],
		BlindingPoint: pubkeys[1],
		Hops: []*lnwire.BlindedHop{
			{
				BlindedNodeID: pubkeys[2],
				EncryptedData: []byte{6, 5, 4},
			},
		},
	}

	// Create one payload with no extra data for the final hop.
	payloadNoFinalHops := &lnwire.OnionMessagePayload{
		ReplyPath:     replyPath,
		EncryptedData: []byte{9, 8, 7},
	}

	// Create another payload with extra data for the final hop that will
	// need to be handled.
	finalHopPayload := &lnwire.FinalHopPayload{
		TLVType: tlv.Type(101),
		Value:   []byte{1, 2, 3},
	}

	payloadWithFinal := &lnwire.OnionMessagePayload{
		ReplyPath:     replyPath,
		EncryptedData: []byte{3, 2, 1},
		FinalHopPayloads: []*lnwire.FinalHopPayload{
			finalHopPayload,
		},
	}

	// Create a payload which we don't have a handler for (the test only
	// registers a handler for payload 101).
	unhandledPayload := &lnwire.OnionMessagePayload{
		ReplyPath: replyPath,
		FinalHopPayloads: []*lnwire.FinalHopPayload{
			{
				TLVType: 103,
			},
		},
	}

	tests := []struct {
		name         string
		msg          lndclient.CustomMessage
		processOnion processOnion
		setupMock    func(*mock.Mock)
		expectedErr  error
	}{
		// TODO: add coverage for decoding errors
		{
			name: "message for our node",
			msg:  *msg,
			// Return a packet indicating that we're the recipient.
			processOnion: func(_ *sphinx.OnionPacket,
				_ *btcec.PublicKey) (*sphinx.ProcessedPacket,
				error) {

				return &sphinx.ProcessedPacket{
					Action: sphinx.ExitNode,
				}, nil
			},
			// We only expect our payload to be decoded, no handling
			// because we don't have final payloads.
			setupMock: func(m *mock.Mock) {
				mockPayloadDecode(m, payloadNoFinalHops, nil)
			},
			expectedErr: nil,
		},
		{
			name: "message for forwarding",
			msg:  *msg,
			// Return a packet indicating that there are more hops.
			processOnion: func(_ *sphinx.OnionPacket,
				_ *btcec.PublicKey) (*sphinx.ProcessedPacket,
				error) {

				return &sphinx.ProcessedPacket{
					Action: sphinx.MoreHops,
				}, nil
			},
			// Return a payload with no extra data for the final
			// hop.
			setupMock: func(m *mock.Mock) {
				mockPayloadDecode(m, payloadNoFinalHops, nil)
			},
			expectedErr: ErrNoForwarding,
		},
		{
			name: "message for forwarding with final payload",
			msg:  *msg,
			// Return a packet indicating that there are more hops.
			processOnion: func(_ *sphinx.OnionPacket,
				_ *btcec.PublicKey) (*sphinx.ProcessedPacket,
				error) {

				return &sphinx.ProcessedPacket{
					Action: sphinx.MoreHops,
				}, nil
			},
			// Return a payload that has data for the final hop,
			// despite this being for an intermediate node.
			setupMock: func(m *mock.Mock) {
				mockPayloadDecode(m, payloadWithFinal, nil)
			},
			expectedErr: ErrFinalPayload,
		},
		{
			name: "invalid message",
			msg:  *msg,
			// Return a packet indicating that there are more hops.
			processOnion: func(_ *sphinx.OnionPacket,
				_ *btcec.PublicKey) (*sphinx.ProcessedPacket,
				error) {

				return &sphinx.ProcessedPacket{
					Action: sphinx.Failure,
				}, nil
			},
			// We'll decode the payload before we check the next action
			// for the packet.
			setupMock: func(m *mock.Mock) {
				mockPayloadDecode(m, payloadWithFinal, nil)
			},
			expectedErr: ErrBadMessage,
		},
		{
			name: "processing failed",
			msg:  *msg,
			// Fail onion processing.
			processOnion: func(_ *sphinx.OnionPacket,
				_ *btcec.PublicKey) (*sphinx.ProcessedPacket,
				error) {

				return nil, mockErr
			},
			// Don't prime our mock because we fail on onion
			// processing.
			setupMock: func(*mock.Mock) {},

			expectedErr: ErrBadOnionBlob,
		},
		{
			name: "final payload handled",
			msg:  *msg,
			processOnion: func(_ *sphinx.OnionPacket,
				_ *btcec.PublicKey) (*sphinx.ProcessedPacket,
				error) {

				return &sphinx.ProcessedPacket{
					Action: sphinx.ExitNode,
				}, nil
			},
			// Setup our mock to return a final payload and to
			// handle it without an error.
			setupMock: func(m *mock.Mock) {
				mockPayloadDecode(m, payloadWithFinal, nil)

				mockMessageHandled(
					m,
					payloadWithFinal.ReplyPath,
					payloadWithFinal.EncryptedData,
					finalHopPayload.Value,
					nil,
				)
			},
		},
		{
			name: "final payload handler error",
			msg:  *msg,
			processOnion: func(_ *sphinx.OnionPacket,
				_ *btcec.PublicKey) (*sphinx.ProcessedPacket,
				error) {

				return &sphinx.ProcessedPacket{
					Action: sphinx.ExitNode,
				}, nil
			},
			// Setup our mock to return a final payload and to
			// handle it without an error.
			setupMock: func(m *mock.Mock) {
				mockPayloadDecode(m, payloadWithFinal, nil)

				mockMessageHandled(
					m,
					payloadWithFinal.ReplyPath,
					payloadWithFinal.EncryptedData,
					finalHopPayload.Value,
					mockErr,
				)
			},
			expectedErr: mockErr,
		},
		{
			name: "final payload no handler",
			msg:  *msg,
			processOnion: func(_ *sphinx.OnionPacket,
				_ *btcec.PublicKey) (*sphinx.ProcessedPacket,
				error) {

				return &sphinx.ProcessedPacket{
					Action: sphinx.ExitNode,
				}, nil
			},
			// Setup our mock to return a final payload but not
			// to handle it because we don't register a handler.
			setupMock: func(m *mock.Mock) {
				mockPayloadDecode(m, unhandledPayload, nil)
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			// Setup a mock and prime it with the test's
			// requirements.
			mock := &handleOnionMesageMock{
				Mock: &mock.Mock{},
			}

			testCase.setupMock(mock.Mock)
			defer mock.AssertExpectations(t)

			handlers := map[tlv.Type]OnionMessageHandler{
				finalHopPayload.TLVType: mock.OnionMessageHandler,
			}

			err := handleOnionMessage(
				testCase.processOnion, mock.DecodePayload,
				testCase.msg, handlers,
			)
			require.True(t, errors.Is(err, testCase.expectedErr))
		})
	}
}

// receiveMessageHandler is the function signature for handlers that drive
// tests for our receive message loop.
type receiveMessageHandler func(*testing.T, chan<- lndclient.CustomMessage,
	chan<- error)

// sendMsg is a helped that sends a custom message into the channel provided,
// failing the test if it is not delivered on time.
func sendMsg(t *testing.T, msgChan chan<- lndclient.CustomMessage,
	msg lndclient.CustomMessage) {

	select {
	case msgChan <- msg:
	case <-time.After(defaultTimeout):
		t.Fatalf("could not send message: %v", msg)
	}
}

// sendErr is a helper that sends an error into the channel provided, failing
// the test if it is not delivered in time.
func sendErr(t *testing.T, errChan chan<- error, err error) {
	select {
	case errChan <- err:
	case <-time.After(defaultTimeout):
		t.Fatalf("could not send error: %v", err)
	}
}

// TestReceiveOnionMessages tests the messenger's receive loop for messages.
func TestReceiveOnionMessages(t *testing.T) {
	privkeys := testutils.GetPrivkeys(t, 1)

	// Create an onion message that is *to our node* that we can use
	// across tests.
	nodePubkey := privkeys[0].PubKey()
	nodeVertex, err := route.NewVertexFromBytes(
		nodePubkey.SerializeCompressed(),
	)
	require.NoError(t, err, "node pubkey")

	msg, err := customOnionMessage(nodeVertex, nil, nil)
	require.NoError(t, err, "custom message")

	mockErr := errors.New("mock")

	tests := []struct {
		name          string
		handler       receiveMessageHandler
		expectedError error
	}{
		{
			name: "message sent",
			handler: func(t *testing.T,
				msgChan chan<- lndclient.CustomMessage,
				errChan chan<- error) {

				sendMsg(t, msgChan, *msg)
			},
		}, {
			name: "non-onion message",
			handler: func(t *testing.T,
				msgChan chan<- lndclient.CustomMessage,
				errChan chan<- error) {

				msg := lndclient.CustomMessage{
					MsgType: 1001,
				}

				sendMsg(t, msgChan, msg)
			},
			expectedError: nil,
		},
		{
			name: "lnd shutdown - messages",
			handler: func(t *testing.T,
				msgChan chan<- lndclient.CustomMessage,
				errChan chan<- error) {

				close(msgChan)
			},
			expectedError: ErrLNDShutdown,
		},
		{
			name: "lnd shutdown - errors",
			handler: func(t *testing.T,
				msgChan chan<- lndclient.CustomMessage,
				errChan chan<- error) {

				close(errChan)
			},
			expectedError: ErrLNDShutdown,
		},
		{
			name: "subscription error",
			handler: func(t *testing.T,
				msgChan chan<- lndclient.CustomMessage,
				errChan chan<- error) {

				sendErr(t, errChan, mockErr)
			},
			expectedError: mockErr,
		},
	}

	for _, testCase := range tests {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			testReceiveOnionMessages(
				t, privkeys[0], testCase.handler,
				testCase.expectedError,
			)
		})
	}
}

func testReceiveOnionMessages(t *testing.T, privkey *btcec.PrivateKey,
	handler receiveMessageHandler, expectedErr error) {

	// Create a simple node key ecdh impl for our messenger.
	nodeKeyECDH := &sphinx.PrivKeyECDH{
		PrivKey: privkey,
	}

	// Setup a mocked lnd and prime it to have SubscribeCustomMessages
	// called.
	lnd := testutils.NewMockLnd()
	defer lnd.Mock.AssertExpectations(t)

	// Create channels to deliver messages and fail if they block for too
	// long.
	var (
		msgChan = make(chan lndclient.CustomMessage)
		errChan = make(chan error)

		shutdownChan    = make(chan error)
		requestShutdown = func(err error) {
			select {
			case shutdownChan <- err:
			case <-time.After(defaultTimeout):
				t.Fatalf("did not shutdown with: %v", err)
			}
		}
	)

	// Set up our mock to return our message channels when we subscribe to
	// custom lnd messages.
	// Note: might be wrong types?
	testutils.MockSubscribeCustomMessages(
		lnd.Mock, msgChan, errChan, nil,
	)

	messenger := NewOnionMessenger(
		&chaincfg.RegressionNetParams, lnd, nodeKeyECDH,
		requestShutdown,
	)
	err := messenger.Start()
	require.NoError(t, err, "start messenger")

	// Shutdown our messenger at the end of the test.
	defer func() {
		err := messenger.Stop()
		require.NoError(t, err, "stop messenger")
	}()

	// Run the specific test's handler.
	handler(t, msgChan, errChan)

	// If we expect to exit with an error, expect it to be surfaced through
	// requesting a graceful shutdown.
	if expectedErr != nil {
		select {
		case err := <-shutdownChan:
			require.True(t, errors.Is(err, expectedErr), "shutdown")

		case <-time.After(defaultTimeout):
			t.Fatal("no shutdown error recieved")
		}
	}
}

// TestHandleRegistration tests registration of handlers for tlv payloads.
func TestHandleRegistration(t *testing.T) {
	var (
		invalidTlv tlv.Type = 10
		validTlv   tlv.Type = 100

		handler = func(*lnwire.ReplyPath, []byte, []byte) error {
			return nil
		}

		nodeKeyECDH = &sphinx.PrivKeyECDH{
			PrivKey: testutils.GetPrivkeys(t, 1)[0],
		}
	)

	// Assert that our test tlv values have the validity we expect.
	require.Nil(t, lnwire.ValidateFinalPayload(validTlv))

	err := lnwire.ValidateFinalPayload(invalidTlv)
	require.True(t, errors.Is(err, lnwire.ErrNotFinalPayload))

	// Setups a mock lnd. We need this to subscribe to incoming messages,
	// even though we're not testing message handling in this test. W
	lnd := testutils.NewMockLnd()
	defer lnd.Mock.AssertExpectations(t)

	// Prime our mock for our startup call, using nil channels because we
	// won't actually deliver messages.
	testutils.MockSubscribeCustomMessages(
		lnd.Mock, nil, nil, nil,
	)

	// Create a messenger, but don't start it yet.
	messenger := NewOnionMessenger(
		&chaincfg.RegressionNetParams, lnd, nodeKeyECDH, nil,
	)

	// Assert the registration fails if we're not started.
	err = messenger.RegisterHandler(validTlv, handler)
	require.True(t, errors.Is(err, ErrNotStarted), "err: %v", err.Error())

	// Start our messenger. We'll shut it down manually later, so we don't
	// defer stop here.
	require.NoError(t, messenger.Start(), "start messenger")

	// Now that we're started, we should be able to register with no issue.
	err = messenger.RegisterHandler(validTlv, handler)
	require.NoError(t, err, "valid tlv register")

	// Try to re-register with the same type, we should fail.
	err = messenger.RegisterHandler(validTlv, handler)
	require.True(t, errors.Is(err, ErrHandlerRegistered))

	// Try to register a handler for an out-of-range tlv type, expect
	// failure.
	err = messenger.RegisterHandler(invalidTlv, handler)
	require.True(t, errors.Is(err, lnwire.ErrNotFinalPayload))

	// Try to de-register our existing handler, we should succeed.
	err = messenger.DeregisterHandler(validTlv)
	require.NoError(t, err)

	// Try to de-register a handler that's no longer registered, we should
	// get an error.
	err = messenger.DeregisterHandler(validTlv)
	require.True(t, errors.Is(err, ErrHandlerNotFound))

	// Shut down our messenger to test registration requests during
	// shutdown.
	require.NoError(t, messenger.Stop(), "stop messenger")

	err = messenger.RegisterHandler(validTlv, handler)
	require.True(t, errors.Is(err, ErrShuttingDown))
}

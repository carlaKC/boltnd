package testutils

import (
	"context"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/lightninglabs/lndclient"
	"github.com/lightningnetwork/lnd/keychain"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/lightningnetwork/lnd/routing/route"
	"github.com/stretchr/testify/mock"
)

// MockLND creates a mock that substitutes for an API connection to lnd.
type MockLND struct {
	*mock.Mock
}

// NewMockLnd creates a lnd mock.
func NewMockLnd() *MockLND {
	return &MockLND{
		Mock: &mock.Mock{},
	}
}

// SendCustomMessage mocks sending a custom message to a peer.
func (m *MockLND) SendCustomMessage(ctx context.Context,
	msg lndclient.CustomMessage) error {

	args := m.Mock.MethodCalled("SendCustomMessage", ctx, msg)
	return args.Error(0)
}

// MockSendAnyCustomMessage primes our mock to return the error provided when
// send custom message is called with any CustomMessage.
func MockSendAnyCustomMessage(m *mock.Mock, err error) {
	m.On(
		"SendCustomMessage", mock.Anything,
		// Our custom messages will be generated with random session
		// keys, so we just assert on type here.
		mock.AnythingOfType("lndclient.CustomMessage"),
	).Once().Return(
		err,
	)
}

// SubscribeCustomMessages mocks subscribing to custom messages from lnd.
func (m *MockLND) SubscribeCustomMessages(ctx context.Context) (
	<-chan lndclient.CustomMessage, <-chan error, error) {

	args := m.Mock.MethodCalled("SubscribeCustomMessages", ctx)

	msgChan := args.Get(0).(<-chan lndclient.CustomMessage)
	errChan := args.Get(1).(<-chan error)

	return msgChan, errChan, args.Error(2)
}

// MockSubscribeCustomMessages primes our mock to return the channels and error
// provided when we subscribe to custom messages.
func MockSubscribeCustomMessages(m *mock.Mock,
	msgChan <-chan lndclient.CustomMessage, errChan <-chan error,
	err error) {

	m.On(
		"SubscribeCustomMessages", mock.Anything,
	).Once().Return(
		msgChan, errChan, err,
	)
}

// GetNodeInfo mocks looking up a node in the public ln graph.
func (m *MockLND) GetNodeInfo(ctx context.Context, pubkey route.Vertex,
	includeChannels bool) (*lndclient.NodeInfo, error) {

	args := m.Mock.MethodCalled("GetNodeInfo", ctx, pubkey, includeChannels)

	node := args.Get(0)
	return node.(*lndclient.NodeInfo), args.Error(1)
}

// MockGetNodeInfo primes our mock to return the info and error provided when
// a call to get node info with the peer/include channels params is made.
func MockGetNodeInfo(m *mock.Mock, pubkey route.Vertex, includeChannels bool,
	info *lndclient.NodeInfo, err error) {

	m.On(
		"GetNodeInfo", mock.Anything, pubkey,
		includeChannels,
	).Once().Return(
		info, err,
	)
}

// ListPeers mocks returning lnd's current set of peers.
func (m *MockLND) ListPeers(ctx context.Context) ([]lndclient.Peer, error) {

	args := m.Mock.MethodCalled("ListPeers", ctx)

	peers := args.Get(0)
	return peers.([]lndclient.Peer), args.Error(1)
}

// MockListPeers primes our mock to return the peers and error specified on a
// call to ListPeers.
func MockListPeers(m *mock.Mock, peers []lndclient.Peer, err error) {
	m.On(
		"ListPeers", mock.Anything,
	).Once().Return(
		peers, err,
	)
}

// Connect mocks a connection to the peer provided.
func (m *MockLND) Connect(ctx context.Context, peer route.Vertex, host string,
	permanent bool) error {

	args := m.Mock.MethodCalled("Connect", ctx, peer, host, permanent)

	return args.Error(0)
}

// MockConnect primes our mock to return error specified on a call to Connect.
func MockConnect(m *mock.Mock, peer route.Vertex, host string, perm bool,
	err error) {

	m.On(
		"Connect", mock.Anything, peer, host, perm,
	).Once().Return(
		err,
	)
}

// GetInfo mocks a call to lnd's getinfo.
func (m *MockLND) GetInfo(ctx context.Context) (*lndclient.Info, error) {
	args := m.Mock.MethodCalled("GetInfo", ctx)

	return args.Get(0).(*lndclient.Info), args.Error(1)
}

// MockGetInfo primes our mock to return the info and error provided when
// GetInfo is called.
func MockGetInfo(m *mock.Mock, info *lndclient.Info, err error) {
	m.On(
		"GetInfo", mock.Anything,
	).Once().Return(
		info, err,
	)
}

// DeriveSharedKey mocks lnd's ECDH operations.
func (m *MockLND) DeriveSharedKey(ctx context.Context,
	ephemeralPubKey *btcec.PublicKey, keyLocator *keychain.KeyLocator) (
	[32]byte, error) {

	args := m.Mock.MethodCalled(
		"DeriveSharedKey", ctx, ephemeralPubKey, keyLocator,
	)

	key := args.Get(0).([32]byte)

	return key, args.Error(1)
}

// MockDeriveSharedKey primes our mock to return the key and error provided
// when derive shared key is called.
func MockDeriveSharedKey(m *mock.Mock, ephemeral *btcec.PublicKey,
	locator *keychain.KeyLocator, key [32]byte, err error) {

	m.On(
		"DeriveSharedKey", mock.Anything, ephemeral, locator,
	).Once().Return(
		key, err,
	)
}

// ListChannels mocks a call to lnd's list channels api.
func (m *MockLND) ListChannels(ctx context.Context, activeOnly,
	publicOnly bool) ([]lndclient.ChannelInfo, error) {

	args := m.Mock.MethodCalled(
		"ListChannels", ctx, activeOnly, publicOnly,
	)

	channels := args.Get(0).([]lndclient.ChannelInfo)
	return channels, args.Error(1)
}

// MockListChannels primes our mock to return the channel list and error
// provided when ListChannels is called.
func MockListChannels(m *mock.Mock, activeOnly, publicOnly bool,
	channels []lndclient.ChannelInfo, err error) {

	m.On(
		"ListChannels", mock.Anything, activeOnly, publicOnly,
	).Once().Return(
		channels, err,
	)
}

// SendPayment mocks dispatch of a payment via lnd.
func (m *MockLND) SendPayment(ctx context.Context,
	request lndclient.SendPaymentRequest) (chan lndclient.PaymentStatus,
	chan error, error) {

	args := m.Mock.MethodCalled(
		"SendPayment", ctx, request,
	)

	payChan := args.Get(0).(chan lndclient.PaymentStatus)
	errChan := args.Get(1).(chan error)

	return payChan, errChan, args.Error(2)
}

// MockSendPayment primes our mock for a sendpayment call.
func MockSendPayment(m *mock.Mock, request lndclient.SendPaymentRequest,
	payChan chan lndclient.PaymentStatus, errChan chan error, err error) {

	m.On(
		"SendPayment", mock.Anything, request,
	).Once().Return(
		payChan, errChan, err,
	)
}

// TrackPayment mocks dispatch of a payment via lnd.
func (m *MockLND) TrackPayment(ctx context.Context, hash lntypes.Hash) (
	chan lndclient.PaymentStatus, chan error, error) {

	args := m.Mock.MethodCalled(
		"TrackPayment", ctx, hash,
	)

	payChan := args.Get(0).(chan lndclient.PaymentStatus)
	errChan := args.Get(1).(chan error)

	return payChan, errChan, args.Error(2)
}

// MockTrackPayment primes our mock for a trackpayment call.
func MockTrackPayment(m *mock.Mock, hash lntypes.Hash,
	payChan chan lndclient.PaymentStatus, errChan chan error, err error) {

	m.On(
		"TrackPayment", mock.Anything, hash,
	).Once().Return(
		payChan, errChan, err,
	)
}

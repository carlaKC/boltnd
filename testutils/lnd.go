package testutils

import (
	"context"

	"github.com/lightninglabs/lndclient"
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

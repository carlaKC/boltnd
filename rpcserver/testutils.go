package rpcserver

import (
	"context"
	"testing"

	"github.com/carlakc/boltnd/lnwire"
	"github.com/carlakc/boltnd/onionmsg"
	"github.com/carlakc/boltnd/testutils"
	"github.com/lightningnetwork/lnd/routing/route"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type serverTest struct {
	t         *testing.T
	server    *Server
	lnd       *testutils.MockLND
	offerMock *offersMock
}

func newServerTest(t *testing.T) *serverTest {
	serverTest := &serverTest{
		t:         t,
		lnd:       testutils.NewMockLnd(),
		offerMock: newOffersMock(),
	}

	var err error
	serverTest.server, err = NewServer(nil)
	require.NoError(t, err, "new server")

	// Override our onion messenger with a mock.
	serverTest.server.onionMsgr = serverTest.offerMock

	return serverTest
}

// start unblocks the server for our test, in lieu of calling the server's
// actual start function.
func (s *serverTest) start() {
	// We don't use our actual start function, because that will fill in
	// a real onion messenger. Instead, we just close our ready channel
	// to unblock server requests.
	close(s.server.ready)
}

// stop shuts down our server, asserting that mocks have been called as
// expected.
func (s *serverTest) stop() {
	s.lnd.Mock.AssertExpectations(s.t)
	s.offerMock.Mock.AssertExpectations(s.t)
}

// offersMock houses a mock for all the external interfaces that the rpcserver
// required. This allows tests to focus on parsing, error handling and response
// formatting.
type offersMock struct {
	*mock.Mock

	// Embed the onion messenger interface so that all functionality is
	// included.
	onionmsg.OnionMessenger
}

func newOffersMock() *offersMock {
	return &offersMock{
		Mock: &mock.Mock{},
	}
}

// SendMessage mocks sending a message.
func (o *offersMock) SendMessage(ctx context.Context, peer route.Vertex,
	payloads []*lnwire.FinalHopPayload) error {

	args := o.Mock.MethodCalled("SendMessage", ctx, peer, payloads)
	return args.Error(0)
}

// mockSendMessage primes our mock to return the error provided when we call
// send message with the peer provided.
func mockSendMessage(m *mock.Mock, peer route.Vertex,
	payloads []*lnwire.FinalHopPayload, err error) {

	m.On(
		"SendMessage", mock.Anything, peer, payloads,
	).Once().Return(
		err,
	)
}

package rpcserver

import (
	"context"
	"testing"

	"github.com/carlakc/boltnd/offersrpc"
	"github.com/carlakc/boltnd/onionmsg"
	"github.com/carlakc/boltnd/testutils"
	"github.com/lightningnetwork/lnd/tlv"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

type serverTest struct {
	t         *testing.T
	server    *Server
	lnd       *testutils.MockLND
	offerMock *offersMock
	routeMock *testutils.MockRouteGenerator
}

func newServerTest(t *testing.T) *serverTest {
	serverTest := &serverTest{
		t:         t,
		lnd:       testutils.NewMockLnd(),
		offerMock: newOffersMock(),
		routeMock: testutils.NewMockRouteGenerator(),
	}

	var err error
	serverTest.server, err = NewServer(nil)
	require.NoError(t, err, "new server")

	// Override our onion messenger with a mock.
	serverTest.server.onionMsgr = serverTest.offerMock

	serverTest.server.routeGenerator = serverTest.routeMock

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
	s.routeMock.Mock.AssertExpectations(s.t)
}

// offersMock houses a mock for all the external interfaces that the rpcserver
// required. This allows tests to focus on parsing, error handling and response
// formatting.
type offersMock struct {
	*mock.Mock

	// Embed the onion messenger interface so that all functionality is
	// included.
	onionmsg.OnionMessenger

	// Embed the server stream interface so that all functionality is
	// included.
	grpc.ServerStream
}

func newOffersMock() *offersMock {
	return &offersMock{
		Mock: &mock.Mock{},
	}
}

// SendMessage mocks sending a message.
func (o *offersMock) SendMessage(ctx context.Context,
	req *onionmsg.SendMessageRequest) error {

	args := o.Mock.MethodCalled(
		"SendMessage", ctx, req,
	)

	return args.Error(0)
}

// mockSendMessage primes our mock to return the error provided when we call
// send message with the peer provided.
func mockSendMessage(m *mock.Mock, req *onionmsg.SendMessageRequest,
	err error) {

	m.On(
		"SendMessage", mock.Anything, req,
	).Once().Return(
		err,
	)
}

// RegisterHandler mocks registering a handler.
func (o *offersMock) RegisterHandler(tlvType tlv.Type,
	handler onionmsg.OnionMessageHandler) error {

	args := o.Mock.MethodCalled("RegisterHandler", tlvType, handler)
	return args.Error(0)
}

// mockRegisterHandler primes our mock to return the error provided when a call
// to register handler with tlvType (and any handler function) is called.
func mockRegisterHandler(m *mock.Mock, tlvType tlv.Type, err error) {
	m.On(
		"RegisterHandler", tlvType, mock.Anything,
	).Once().Return(
		err,
	)
}

// DeregisterHandler mocks deregistering a handler.
func (o *offersMock) DeregisterHandler(tlvType tlv.Type) error {
	args := o.Mock.MethodCalled("DeregisterHandler", tlvType)
	return args.Error(0)
}

// mockDeregisterHandler primes our mock to return the error provided when a
// call to dereigster handler with tlvType is made.
func mockDeregisterHandler(m *mock.Mock, tlvType tlv.Type, err error) {
	m.On(
		"DeregisterHandler", tlvType,
	).Once().Return(
		err,
	)
}

// Context mocks querying a grpc stream for its context.
func (o *offersMock) Context() context.Context {
	args := o.Mock.MethodCalled("Context")
	return args.Get(0).(context.Context)
}

// mockContext primes our mock to return the context provided.
func mockContext(m *mock.Mock, ctx context.Context) {
	m.On("Context").Once().Return(ctx)
}

// Send mocks sending responses into an onion payload subscription stream.
func (o *offersMock) Send(resp *offersrpc.SubscribeOnionPayloadResponse) error {
	args := o.Mock.MethodCalled("Send", resp)
	return args.Error(0)
}

// mockOnionPayloadSend primes our mock to be called to send the request
// provided into an onion payload subscription stream.
func mockOnionPayloadSend(m *mock.Mock,
	resp *offersrpc.SubscribeOnionPayloadResponse, err error) {

	m.On(
		"Send", resp,
	).Once().Return(
		err,
	)
}

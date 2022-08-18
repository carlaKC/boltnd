package rpcserver

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/carlakc/boltnd/offersrpc"
	"github.com/lightningnetwork/lnd/tlv"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type subscribeOnionPayloadTestCase struct {
	name      string
	setupMock func(*mock.Mock)
	request   *offersrpc.SubscribeOnionPayloadRequest
	testFunc  func(s *serverTest)
	errCode   codes.Code
}

// TestSubscribeOnionPayload tests management of onion payload subscriptions.
func TestSubscribeOnionPayload(t *testing.T) {
	var (
		tlvType tlv.Type = 100
		mockErr          = errors.New("mock err")

		req = &offersrpc.SubscribeOnionPayloadRequest{
			TlvType: uint64(tlvType),
		}

		ctxc, cancel = context.WithCancel(context.Background())
	)

	tests := []*subscribeOnionPayloadTestCase{
		{
			name: "bad tlv type",
			request: &offersrpc.SubscribeOnionPayloadRequest{
				TlvType: 2,
			},
			errCode: codes.InvalidArgument,
		},
		{
			name: "register handler fails",
			setupMock: func(m *mock.Mock) {
				mockContext(m, context.Background())
				mockRegisterHandler(m, tlvType, mockErr)
			},
			request: req,
			errCode: codes.Unavailable,
		},
		{
			name: "server shutdown",
			setupMock: func(m *mock.Mock) {
				mockContext(m, context.Background())
				mockRegisterHandler(m, tlvType, nil)
				mockDeregisterHandler(m, tlvType, nil)

			},
			request: req,
			testFunc: func(s *serverTest) {
				// Shutdown the server.
				close(s.server.quit)
			},
			errCode: codes.Aborted,
		},
		{
			name: "context canceled",
			setupMock: func(m *mock.Mock) {
				// Use a context that we can cancel.
				mockContext(m, ctxc)

				// Assert that we register a handler.
				mockRegisterHandler(m, tlvType, nil)

				// Assert that we deregister on exit.
				mockDeregisterHandler(m, tlvType, nil)
			},
			request: req,
			testFunc: func(s *serverTest) {
				// Cancel our context to force the stream
				// to exit.
				cancel()
			},
			errCode: codes.Canceled,
		},
	}

	for _, testCase := range tests {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			testSubscribeOnionPayload(t, testCase)
		})
	}
}

func testSubscribeOnionPayload(t *testing.T,
	testCase *subscribeOnionPayloadTestCase) {

	s := newServerTest(t)
	s.start()
	defer s.stop()

	// All of our tests begin with a call to Context (in waitForReady).
	// We always want this to pass, so we just use a background context,
	// so that there are no race conditions where we're testing context
	// cancellation at a later stage and the test exits earlier than we
	// want.
	mockContext(s.offerMock.Mock, context.Background())

	if testCase.setupMock != nil {
		testCase.setupMock(s.offerMock.Mock)
	}

	// Our subscription function blocks, so we run it in a goroutine.
	errChan := make(chan error)
	go func() {
		errChan <- s.server.SubscribeOnionPayload(
			testCase.request, s.offerMock,
		)
	}()

	// If we have any additional logic, run it now.
	if testCase.testFunc != nil {
		testCase.testFunc(s)
	}

	select {
	case err := <-errChan:
		// If we're expecting an error, assert that it has the correct
		// code and then exit - there's no further testing to do.
		if testCase.errCode != codes.OK {
			status, ok := status.FromError(err)
			require.True(t, ok)
			require.Equal(t, testCase.errCode, status.Code())

			return
		}

		// If we weren't expecting an error, require that it was nil.
		require.Nil(t, err)

	case <-time.After(time.Second * 5):
		t.Fatal("timeout waiting for test")
	}
}

// TestHandleSubscribOnionPayload tests the underlying handler for onion
// message payload subscriptions. This lower level test allows us to pass our
// own incoming channel in, to allow easy delivery of incoming messages.
func TestHandleSubscribOnionPayload(t *testing.T) {
	var (
		ctx, cancel          = context.WithCancel(context.Background())
		tlvType     tlv.Type = 100

		quit     = make(chan struct{})
		incoming = make(chan onionPayloadResponse)

		s = newServerTest(t)
	)

	s.start()
	defer s.stop()

	// Setup our mock to register our handler, and de-register it when
	// we're done.
	mockRegisterHandler(s.offerMock.Mock, tlvType, nil)
	mockDeregisterHandler(s.offerMock.Mock, tlvType, nil)

	// Our function blocks, so we run it in a goroutine.
	errChan := make(chan error)
	go func() {
		errChan <- handleSubscribeOnionPayload(
			ctx, tlvType, incoming, quit,
			s.offerMock, s.offerMock.Send,
		)
	}()

	// Setup our mock to successfully send a payload, then deliver it for
	// processing via our incoming channel.
	resp := &offersrpc.SubscribeOnionPayloadResponse{
		Value: []byte{6, 9},
	}
	mockOnionPayloadSend(s.offerMock.Mock, resp, nil)
	incoming <- onionPayloadResponse{
		payload: resp.Value,
	}

	// To shutdown out test, cancel our context and assert that we exit
	// with a canceled code.
	cancel()

	select {
	case err := <-errChan:
		require.Error(t, err, "expect canceled")

		status, ok := status.FromError(err)
		require.True(t, ok)
		require.Equal(t, codes.Canceled, status.Code())

	case <-time.After(time.Second * 5):
	}

}

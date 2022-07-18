package rpcserver

import (
	"context"
	"testing"

	"github.com/carlakc/boltnd/offersrpc"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestDecodeOffer tests the rpc mechanics of decoding offers - validation,
// parsing and response forming. This test does not cover offer decoding itself,
// which should be covered by the offers package, so it uses only valid offers.
func TestDecodeOffer(t *testing.T) {
	tests := []struct {
		name    string
		request *offersrpc.DecodeOfferRequest
		success bool
		errCode codes.Code
	}{
		{
			name:    "no offer supplied",
			request: &offersrpc.DecodeOfferRequest{},
			success: false,
			errCode: codes.InvalidArgument,
		},
	}

	for _, testCase := range tests {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			s := newServerTest(t)
			s.start()
			defer s.stop()

			// Send the test's request to the server.
			_, err := s.server.DecodeOffer(
				context.Background(), testCase.request,
			)
			require.Equal(t, testCase.success, err == nil)

			// If our test was a success, we don't need to test our
			// error further.
			if testCase.success {
				return
			}

			// If we expect a failure, assert that it has the error
			// code we want.
			require.NotNil(t, err, "invalid pubkey send")

			status, ok := status.FromError(err)
			require.True(t, ok, "expected coded error")
			require.Equal(t, status.Code(), testCase.errCode)
		})
	}
}

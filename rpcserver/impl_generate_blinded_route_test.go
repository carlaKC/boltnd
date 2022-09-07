package rpcserver

import (
	"context"
	"math"
	"testing"

	"github.com/carlakc/boltnd/offersrpc"
	"github.com/carlakc/boltnd/testutils"
	sphinx "github.com/lightningnetwork/lightning-onion"
	lndwire "github.com/lightningnetwork/lnd/lnwire"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestGenerateBlindedRoute tests generation of blinded routes.
func TestGenerateBlindedRoute(t *testing.T) {
	path := &sphinx.BlindedPath{
		IntroductionPoint: testutils.GetPubkeys(t, 1)[0],
	}

	tests := []struct {
		name      string
		setupMock func(m *mock.Mock)
		request   *offersrpc.GenerateBlindedRouteRequest
		errCode   codes.Code
	}{

		{
			name: "bad feature bit",
			request: &offersrpc.GenerateBlindedRouteRequest{
				Features: []uint64{
					math.MaxUint64,
				},
			},
			errCode: codes.InvalidArgument,
		},
		{
			name:    "no features",
			request: &offersrpc.GenerateBlindedRouteRequest{},
			setupMock: func(m *mock.Mock) {
				testutils.MockBlindedRoute(
					m, []lndwire.FeatureBit{}, path, nil,
				)
			},
		},
		{
			name: "ok features",
			request: &offersrpc.GenerateBlindedRouteRequest{
				Features: []uint64{
					uint64(lndwire.AMPOptional),
				},
			},
			setupMock: func(m *mock.Mock) {
				testutils.MockBlindedRoute(
					m, []lndwire.FeatureBit{
						lndwire.AMPOptional,
					}, path, nil,
				)
			},
		},
	}

	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			s := newServerTest(t)
			s.start()
			defer s.stop()

			if testCase.setupMock != nil {
				testCase.setupMock(s.routeMock.Mock)
			}

			_, err := s.server.GenerateBlindedRoute(
				context.Background(), testCase.request,
			)

			if testCase.errCode == codes.OK {
				require.Nil(t, err)
				return
			}

			status, ok := status.FromError(err)
			require.True(t, ok, "expected err code")
			require.Equal(t, testCase.errCode, status.Code())
		})
	}
}

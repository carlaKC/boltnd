package rpcserver

import (
	"testing"

	"github.com/carlakc/boltnd/lnwire"
	"github.com/carlakc/boltnd/offersrpc"
	"github.com/carlakc/boltnd/testutils"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestParseReplyPath tests conversion of rpc blinded paths to reply paths.
func TestParseReplyPath(t *testing.T) {
	var (
		pubkeys = testutils.GetPubkeys(t, 3)
		pubkey0 = pubkeys[0].SerializeCompressed()
		pubkey1 = pubkeys[1].SerializeCompressed()
		pubkey2 = pubkeys[2].SerializeCompressed()
	)

	tests := []struct {
		name     string
		path     *offersrpc.BlindedPath
		errCode  codes.Code
		expected *lnwire.ReplyPath
	}{
		{
			name:    "no introduction point",
			path:    &offersrpc.BlindedPath{},
			errCode: codes.InvalidArgument,
		},
		{
			name: "no blinding point",
			path: &offersrpc.BlindedPath{
				IntroductionNode: pubkey0,
			},
			errCode: codes.InvalidArgument,
		},
		{
			name: "invalid hop",
			path: &offersrpc.BlindedPath{
				IntroductionNode: pubkey0,
				BlindingPoint:    pubkey1,
				Hops: []*offersrpc.BlindedHop{
					{
						// No node ID for hop.
						EncrypedData: []byte{1, 2, 3},
					},
				},
			},
			errCode: codes.InvalidArgument,
		},
		{
			name: "valid path",
			path: &offersrpc.BlindedPath{
				IntroductionNode: pubkey0,
				BlindingPoint:    pubkey1,
				Hops: []*offersrpc.BlindedHop{
					{
						BlindedNodeId: pubkey2,
						EncrypedData:  []byte{3, 2, 1},
					},
				},
			},
			errCode: codes.OK,
			expected: &lnwire.ReplyPath{
				FirstNodeID:   pubkeys[0],
				BlindingPoint: pubkeys[1],
				Hops: []*lnwire.BlindedHop{
					{
						BlindedNodeID: pubkeys[2],
						EncryptedData: []byte{3, 2, 1},
					},
				},
			},
		},
	}

	for _, testCase := range tests {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			path, err := parseReplyPath(testCase.path)
			if testCase.errCode == codes.OK {
				require.Nil(t, err)
				require.Equal(t, testCase.expected, path)
				return
			}

			require.NotNil(t, err)
			status, ok := status.FromError(err)
			require.True(t, ok)
			require.Equal(t, status.Code(), testCase.errCode)
		})
	}
}

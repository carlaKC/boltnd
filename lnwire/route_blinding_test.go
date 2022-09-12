package lnwire

import (
	"testing"

	"github.com/carlakc/boltnd/testutils"
	"github.com/stretchr/testify/require"
)

// TestRouteBlindingEncoding tests encoding of the TLVs used in route blinding
// blobs.
func TestRouteBlindingEncoding(t *testing.T) {
	pubkeys := testutils.GetPubkeys(t, 1)

	tests := []struct {
		name string
		data *BlindedRouteData
	}{
		{
			name: "node id",
			data: &BlindedRouteData{
				NextNodeID: pubkeys[0],
			},
		},
		{
			name: "blinding override",
			data: &BlindedRouteData{
				NextBlindingOverride: pubkeys[0],
			},
		},
	}

	for _, testCase := range tests {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			bytes, err := EncodeBlindedRouteData(testCase.data)
			require.NoError(t, err, "encode data")

			actual, err := DecodeBlindedRouteData(bytes)
			require.NoError(t, err, "decode data")

			require.Equal(t, testCase.data, actual)
		})
	}
}

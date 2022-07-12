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

	original := &BlindedRouteData{
		NextNodeID: pubkeys[0],
	}

	bytes, err := EncodeBlindedRouteData(original)
	require.NoError(t, err, "encode data")

	actual, err := DecodeBlindedRouteData(bytes)
	require.NoError(t, err, "decode data")

	require.Equal(t, original, actual)
}

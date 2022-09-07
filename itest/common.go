package itest

import (
	"testing"

	"github.com/carlakc/boltnd/offersrpc"
	"github.com/stretchr/testify/require"
)

// assertBlindedPathEqual asserts that two blinded paths are equal.
func assertBlindedPathEqual(t *testing.T, expected,
	actual *offersrpc.BlindedPath) {

	require.Equal(t, expected.IntroductionNode, actual.IntroductionNode,
		"introduction")

	require.Equal(t, expected.BlindingPoint, actual.BlindingPoint,
		"blinding point")

	require.Equal(t, expected.IntroductionEncryptedData,
		actual.IntroductionEncryptedData, "introduction data")

	require.Equal(t, len(expected.Hops), len(actual.Hops), "hop count")

	for i, hop := range expected.Hops {
		require.Equal(t, hop.BlindedNodeId,
			actual.Hops[i].BlindedNodeId, "blinded node id", i)

		require.Equal(t, hop.EncryptedData,
			actual.Hops[i].EncryptedData, "encrypted data", i)
	}
}

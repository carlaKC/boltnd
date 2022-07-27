package lnwire

import (
	"bytes"
	"testing"

	"github.com/carlakc/boltnd/testutils"
	"github.com/stretchr/testify/require"
)

// TestBlindedHopEncoding tests encoding and decoding of individual blinded
// hops.
func TestBlindedHopEncoding(t *testing.T) {
	var (
		pubkey = testutils.GetPubkeys(t, 1)[0]

		encodedHop = &BlindedHop{
			BlindedNodeID: pubkey,
			EncryptedData: []byte{1, 2, 3},
		}

		w = new(bytes.Buffer)
		b [8]byte
	)

	err := encodeBlindedHop(w, encodedHop, &b)
	require.NoError(t, err, "encode")

	encodedBytes := w.Bytes()
	encodedBytesLen := len(encodedBytes)

	r := bytes.NewReader(encodedBytes)
	decodedHop := &BlindedHop{}
	_, err = decodeBlindedHop(r, decodedHop, &b, uint64(encodedBytesLen))
	require.NoError(t, err, "decode")

	require.Equal(t, encodedHop, decodedHop, "hops differ")
}

package lnwire

import (
	"bytes"
	"testing"

	"github.com/carlakc/boltnd/testutils"
	"github.com/stretchr/testify/require"
)

// mockHops returns three mocked blinded hops, the final one with no encrypted
// data.
func mockHops(t *testing.T) []*BlindedHop {
	pubkeys := testutils.GetPubkeys(t, 3)

	return []*BlindedHop{
		{
			BlindedNodeID: pubkeys[0],
			EncryptedData: []byte{1, 2, 3},
		},
		{
			BlindedNodeID: pubkeys[1],
			EncryptedData: []byte{4, 5, 6},
		},
		{
			BlindedNodeID: pubkeys[2],
			EncryptedData: []byte{},
		},
	}
}

// TestReplyPathEncoding tests encoding and decoding of onion message reply
// paths.
func TestReplyPathEncoding(t *testing.T) {
	var (
		pubkeys = testutils.GetPubkeys(t, 2)

		hops = mockHops(t)

		// For reuse encoding/decoding.
		b [8]byte
	)

	tests := []struct {
		name    string
		encoded *ReplyPath
	}{
		{
			name: "multiple hops",
			encoded: &ReplyPath{
				FirstNodeID:   pubkeys[0],
				BlindingPoint: pubkeys[1],
				Hops: []*BlindedHop{
					hops[0], hops[1],
				},
			},
		},
		{
			name: "no hops",
			encoded: &ReplyPath{
				FirstNodeID:   pubkeys[0],
				BlindingPoint: pubkeys[1],
			},
		},
		{
			name: "hop without data",
			encoded: &ReplyPath{
				FirstNodeID:   pubkeys[0],
				BlindingPoint: pubkeys[1],
				Hops: []*BlindedHop{
					hops[0], hops[1], hops[2],
				},
			},
		},
	}

	for _, testCase := range tests {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			w := new(bytes.Buffer)
			err := encodeReplyPath(w, testCase.encoded, &b)
			require.NoError(t, err)

			encodedBytes := w.Bytes()
			encodedBytesLen := len(encodedBytes)

			decoded := &ReplyPath{}
			r := bytes.NewReader(encodedBytes)

			err = decodeReplyPath(
				r, decoded, &b, uint64(encodedBytesLen),
			)
			require.NoError(t, err)
			require.Equal(t, testCase.encoded, decoded)
		})
	}
}

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

package lnwire

import (
	"bytes"
	"errors"
	"testing"

	"github.com/carlakc/boltnd/testutils"
	"github.com/lightningnetwork/lnd/tlv"
	"github.com/stretchr/testify/require"
)

// TestOnionPayloadEncoding tests encoding and decoding of onion message
// payloads.
func TestOnionPayloadEncoding(t *testing.T) {
	pubkeys := testutils.GetPubkeys(t, 4)

	encoded := &OnionMessagePayload{
		ReplyPath: &ReplyPath{
			FirstNodeID:   pubkeys[0],
			BlindingPoint: pubkeys[1],
			Hops:          mockHops(t),
		},
		EncryptedData: []byte{1, 2},
	}

	encodedBytes, err := EncodeOnionMessagePayload(encoded)
	require.NoError(t, err, "encode payload")

	decoded, err := DecodeOnionMessagePayload(encodedBytes)
	require.NoError(t, err, "decode paylaod")

	require.Equal(t, encoded, decoded, "payloads")
}

// TestOnionPayloadFinalHop tests decoding of onion messages that have final
// hop payload tlvs that our code is not familiar with, and filtering out of
// unknown out-of-range values.
func TestOnionPayloadFinalHop(t *testing.T) {
	pubkeys := testutils.GetPubkeys(t, 2)

	finalHopPayload := &FinalHopPayload{
		// Make our tlv type odd so that we don't fail on an unknown
		// even record.
		TLVType: finalHopPayloadStart + 1,
		Value:   []byte{1, 2, 3},
	}
	encoded := &OnionMessagePayload{
		ReplyPath: &ReplyPath{
			FirstNodeID:   pubkeys[0],
			BlindingPoint: pubkeys[1],
			Hops:          mockHops(t),
		},
		EncryptedData: []byte{1, 2},
		FinalHopPayloads: []*FinalHopPayload{
			finalHopPayload,
		},
	}

	// Create another unknown record, but this one is not in the range of
	// records reserved for the final hop.
	filteredRecordType := finalHopPayloadStart - 1
	filteredRecordValue := []byte{3, 2, 1}

	// Manually encode our onion payload, because our encoding won't
	// include the record we want to test filtering out on decode.
	records := []tlv.Record{
		encoded.ReplyPath.record(),
		tlv.MakePrimitiveRecord(
			encryptedDataTLVType, &encoded.EncryptedData,
		),
		tlv.MakePrimitiveRecord(
			filteredRecordType, &filteredRecordValue,
		),
		tlv.MakePrimitiveRecord(
			finalHopPayload.TLVType, &finalHopPayload.Value,
		),
	}

	stream, err := tlv.NewStream(records...)
	require.NoError(t, err, "new stream")

	b := new(bytes.Buffer)
	require.NoError(t, stream.Encode(b), "encode")

	decoded, err := DecodeOnionMessagePayload(b.Bytes())
	require.NoError(t, err, "decode")

	// Assert that our final decoded payload contains the final hop
	// payload, but not the out-of-range, unknown value odd tlv.
	require.Equal(t, encoded, decoded, "payloads")

	// Add a tlv that is outside of the final hop range to our final hop
	// payloads and assert that we fail to encode the payload.
	outOfRange := &FinalHopPayload{
		TLVType: finalHopPayloadStart - 1,
		Value:   []byte{4, 5, 6},
	}
	encoded.FinalHopPayloads = append(encoded.FinalHopPayloads, outOfRange)

	// Assert that we fail when out of range tlvs are added to our set of
	// final hop payloads.
	_, err = EncodeOnionMessagePayload(encoded)
	require.True(t, errors.Is(err, ErrNotFinalPayload))
}

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

package offers

import (
	"bytes"
	"testing"

	"github.com/lightningnetwork/lnd/tlv"
	"github.com/stretchr/testify/require"
)

// mockTLVEncode is a mocked encoding function for TLV records used for testing.
// It just writes the record's type for simplicity. This allows our mocked
// record types to have nil encoders / decoders.
func mockTLVEncode(record tlv.Record, b [8]byte) ([]byte, error) {
	w := new(bytes.Buffer)

	if err := tlv.WriteVarInt(w, uint64(record.Type()), &b); err != nil {
		return nil, err
	}

	return w.Bytes(), nil
}

// TestCreateTLVLeaves tests creation of tlv and nonce leaves, and filtering out
// of signature tlvs.
func TestCreateTLVLeaves(t *testing.T) {
	var (
		// Create two non-sig records with an arbitrary type.
		record1 = tlv.MakeStaticRecord(
			10, nil, 0, nil, nil,
		)

		record2 = tlv.MakeStaticRecord(
			20, nil, 0, nil, nil,
		)

		// Create a non-sig record that is _after_ the signature tlv
		// type range.
		record3 = tlv.MakeStaticRecord(
			tlv.Type(signatureFieldEnd)+1, nil, 0, nil, nil,
		)

		// Create a sig record at the beginning of our range.
		sig = tlv.MakeStaticRecord(
			signatureFieldStart, nil, 0, nil, nil,
		)

		// Re-use for tlv encoding.
		b [8]byte
	)

	record1Bytes, err := mockTLVEncode(record1, b)
	require.NoError(t, err, "record 1 encode")

	record2Bytes, err := mockTLVEncode(record2, b)
	require.NoError(t, err, "record 2 encode")

	record3Bytes, err := mockTLVEncode(record3, b)
	require.NoError(t, err, "record 3 encode")

	// Lazy combine all of our TLVs.
	allBytes := append(record1Bytes, record2Bytes...)
	allBytes = append(allBytes, record3Bytes...)

	record1Leaf := &TLVLeaf{
		Tag:   TLVTag,
		Value: record1Bytes,
	}

	record1Nonce := &TLVLeaf{
		Tag:   append(NonceTag, allBytes...),
		Value: record1Bytes,
	}

	record2Leaf := &TLVLeaf{
		Tag:   TLVTag,
		Value: record2Bytes,
	}

	record2Nonce := &TLVLeaf{
		Tag:   append(NonceTag, allBytes...),
		Value: record2Bytes,
	}

	record3Leaf := &TLVLeaf{
		Tag:   TLVTag,
		Value: record3Bytes,
	}

	record3Nonce := &TLVLeaf{
		Tag:   append(NonceTag, allBytes...),
		Value: record3Bytes,
	}

	tests := []struct {
		name     string
		records  []tlv.Record
		expected []*TLVLeaf
	}{
		{
			name: "all records, out of order",
			records: []tlv.Record{
				record2, record1, record3,
			},
			expected: []*TLVLeaf{
				record1Leaf, record1Nonce,
				record2Leaf, record2Nonce,
				record3Leaf, record3Nonce,
			},
		},
		{
			name: "signature field excluded",
			records: []tlv.Record{
				record1, sig,
			},
			expected: []*TLVLeaf{
				// Our nonce leaf only has our own bytes
				// because we have a single record.
				record1Leaf, {
					Tag: append(
						NonceTag, record1Bytes...,
					),
					Value: record1Bytes,
				},
			},
		},
	}

	for _, testCase := range tests {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			actual, err := CreateTLVLeaves(
				testCase.records, mockTLVEncode,
			)
			require.NoError(t, err, "create leaves")

			require.Equal(t, testCase.expected, actual, "leaves")
		})
	}
}

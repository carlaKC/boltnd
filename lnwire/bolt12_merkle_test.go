package lnwire

import (
	"bytes"
	"encoding/hex"
	"errors"
	"io"
	"testing"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
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

// mockNode is a simple implementation of the node interface for testing.
type mockNode []byte

// Compile time check that mockNode implements the mock interface.
var _ node = (*mockNode)(nil)

// TaggedHash implements the node interface on our mock, simply copying our
// value into a chainhash type.
func (m *mockNode) TaggedHash() chainhash.Hash {
	var hash chainhash.Hash
	copy(hash[:], []byte(*m))

	return hash
}

// TestOrderNode tests ordering of left and right nodes in our tree.
func TestOrderNodes(t *testing.T) {
	var (
		less = mockNode([]byte{1})
		more = mockNode([]byte{2})
		same = mockNode([]byte{2})
	)

	tests := []struct {
		name          string
		left          *mockNode
		right         *mockNode
		expectedLeft  chainhash.Hash
		expectedRight chainhash.Hash
	}{
		{
			name:          "correct order",
			left:          &less,
			right:         &more,
			expectedLeft:  less.TaggedHash(),
			expectedRight: more.TaggedHash(),
		},
		{
			name:          "switch order",
			left:          &more,
			right:         &less,
			expectedLeft:  less.TaggedHash(),
			expectedRight: more.TaggedHash(),
		},
		{
			name:          "equal",
			left:          &more,
			right:         &same,
			expectedLeft:  more.TaggedHash(),
			expectedRight: same.TaggedHash(),
		},
	}

	for _, testCase := range tests {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			actualLeft, actualRight := orderNodes(
				testCase.left, testCase.right,
			)

			require.Equal(t, testCase.expectedLeft, actualLeft)
			require.Equal(t, testCase.expectedRight, actualRight)
		})
	}
}

// TestCreateBranches tests creating the initial set of branches from our node
// leaves.
func TestCreateBranches(t *testing.T) {
	var (
		// Create some leaves to use as input. Since leaves are ordered
		// by TaggedHash, the values were pre-computed for this test to
		// figure out what ordering we'll have.

		// Precomputed TaggedHash()
		// 723ccb853bd6827ed49025180a48c6e4acce9995168d49a0872e50ac1a98b06d
		leaf1 = &TLVLeaf{
			Value: []byte{1},
		}
		leaf1Hash = leaf1.TaggedHash()

		// Precomputed TaggedHash()
		// b6840751cf95ef9997aa6b2f84c8cdb1576dc83a806b62fd0e0ce1ab718d64e4
		leaf2 = &TLVLeaf{
			Value: []byte{2},
		}
		leaf2Hash = leaf2.TaggedHash()

		// Precomputed TaggedHash()
		// 213dbeafa47125ba515b9efe99334ceda5d3f75d4b4b48c4aa1fa6d704abfc18
		leaf3 = &TLVLeaf{
			Value: []byte{3},
		}
		leaf3Hash = leaf3.TaggedHash()

		// Precomputed TaggedHash()
		// 213dbeafa47125ba515b9efe99334ceda5d3f75d4b4b48c4aa1fa6d704abfc18
		leaf4 = &TLVLeaf{
			Value: []byte{3},
		}
		leaf4Hash = leaf4.TaggedHash()
	)

	// Assert that the ordering we expect for our tagged hash values is
	// as expected:
	//
	// leaf1 < leaf2
	require.Equal(t, -1, bytes.Compare(leaf1Hash[:], leaf2Hash[:]))

	// leaf3 = leaf4
	require.Equal(t, 0, bytes.Compare(leaf3Hash[:], leaf4Hash[:]))

	tests := []struct {
		name     string
		leaves   []*TLVLeaf
		expected []*TLVBranch
		err      error
	}{
		{
			name: "odd leaf count",
			leaves: []*TLVLeaf{
				leaf1, leaf2, leaf3,
			},
			err: ErrOddLeafNodes,
		},
		{
			name: "leaves need ordering",
			leaves: []*TLVLeaf{
				// Leaf1's hash is less than leaf2, so they
				// should be switched.
				leaf2, leaf1, leaf3, leaf4,
			},
			expected: []*TLVBranch{
				&TLVBranch{
					left:  leaf1.TaggedHash(),
					right: leaf2.TaggedHash(),
				},
				&TLVBranch{
					left:  leaf3.TaggedHash(),
					right: leaf4.TaggedHash(),
				},
			},
		},
	}

	for _, testCase := range tests {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			actual, err := CreateTLVBranches(testCase.leaves)
			require.True(t, errors.Is(err, testCase.err))
			require.Equal(t, testCase.expected, actual)
		})
	}
}

// TestCalculateRoot tests calculation of a merkle root from a set of branches.
func TestCalculateRoot(t *testing.T) {
	// Setup chainhash values and copy in bytes.
	var value1, value2, value3, value4 chainhash.Hash
	var b1, b2, b3, b4 = [32]byte{1}, [32]byte{2}, [32]byte{3}, [32]byte{4}

	require.NoError(t, value1.SetBytes(b1[:]))
	require.NoError(t, value2.SetBytes(b2[:]))
	require.NoError(t, value3.SetBytes(b3[:]))
	require.NoError(t, value4.SetBytes(b4[:]))

	var (
		// Precomputed TaggedHash:
		// a84d404e10e1f3ab329b5d3caf4f6f8c81f7071744cf47c7f1c5ac59b8fdc333
		branch1 = &TLVBranch{
			left:  value1,
			right: value2,
		}
		branch1Hash = branch1.TaggedHash()

		// Precomputed TaggedHash:
		// ccfe70ec210989c15861fe0155b634e1ce082d75c9e0455c75a863d7a263e108
		branch2 = &TLVBranch{
			left:  value3,
			right: value4,
		}
		branch2Hash = branch2.TaggedHash()

		// Precomputed TaggedHash:
		// 8907af19a8d1ebaa00dc3175f6f03cfd060f9db8d23a3f2d33d84c7c83a9156b
		comboBranch = &TLVBranch{
			left:  branch1Hash,
			right: branch2Hash,
		}
		comboBranchHash = comboBranch.TaggedHash()

		// Precomputed TaggedHash:
		// ccfe70ec210989c15861fe0155b634e1ce082d75c9e0455c75a863d7a263e108
		branch3 = &TLVBranch{
			left:  value3,
			right: value4,
		}
		branch3Hash = branch3.TaggedHash()

		// finalBranch is the final combination of all our branches.
		// branch 1     branch 2     branch 3      (branch 1 < branch 2)
		//    \            /            |
		//      comboBranch          branch 3      (combo < branch 3)
		//           \                /
		//              finalBranch
		// Precomputed hash:
		// 4f1430127c41bc0d4495505998c7e30f67d2a120f8d1f251469cafb3e03a88fa
		finalBranch = &TLVBranch{
			left:  comboBranchHash,
			right: branch3Hash,
		}
		finalBranchHash = finalBranch.TaggedHash()
	)

	// Assert that the ordering we expect for our tagged hash values is
	// as expected:
	//
	// branch 1 < branch 2
	require.Equal(t, -1, bytes.Compare(branch1Hash[:], branch2Hash[:]))

	// branch 2 = branch 3
	require.Equal(t, 0, bytes.Compare(branch2Hash[:], branch3Hash[:]))

	// The combination of branch 1 + branch 2 < branch 3
	require.Equal(t, -1, bytes.Compare(comboBranchHash[:], branch3Hash[:]))

	tests := []struct {
		name     string
		branches []*TLVBranch
		expected chainhash.Hash
	}{
		{
			name: "odd branch count",
			branches: []*TLVBranch{
				branch1, branch2, branch3,
			},
			expected: finalBranchHash,
		},
		{
			name: "even branch count",
			branches: []*TLVBranch{
				branch1, branch2,
			},
			expected: comboBranchHash,
		},
	}

	for _, testCase := range tests {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			actual := CalculateRoot(testCase.branches)
			require.Equal(t, testCase.expected, actual)
		})
	}
}

func makeTestRecord(t *testing.T, tlvType tlv.Type, length uint64,
	specStr string, b [8]byte) tlv.Record {

	// We're going to trim the type and length off string provided by the
	// spec, so we expect at least 4 chars.
	require.Greater(t, len(specStr), 4, "trim type/length")

	value := specStr[4:]
	recordBytes, err := hex.DecodeString(value)
	require.NoError(t, err, "value bytes")

	// Just write the value as provided by the spec, we don't actually
	// care how it's encoded.
	encode := func(w io.Writer, _ interface{}, _ *[8]byte) error {
		_, err := w.Write(recordBytes)
		return err
	}
	// We can have a nil decode function because we don't need it here.
	record := tlv.MakeStaticRecord(
		tlvType, nil, length, encode, nil,
	)

	// Assert that we'll get back to our spec string when we encode the
	// record.
	specBytes, err := hex.DecodeString(specStr)
	require.NoError(t, err, "decode spec")

	encodedRecord, err := encodeTLV(record, b)
	require.NoError(t, err, "encode record")

	require.Equal(t, specBytes, encodedRecord)

	return record
}

// TestMerkleRoot tests calculation of merkle root for a set of TLVs. The values
// in this test are copied from bolt12/merkle-test.json in the spec. The json
// file does not have the individual TLV values (only all TLVs and leaf values)
// so this test "reconstructs" individual TLV encodings from the test.
func TestMerkleRoot(t *testing.T) {
	const (
		tlv1Type    tlv.Type = 1
		tlv1Length           = 2
		tlv1SpecStr          = "010203e8"

		tlv2Type    tlv.Type = 2
		tlv2Length           = 8
		tlv2SpecStr          = "02080000010000020003"

		tlv3Type    tlv.Type = 3
		tlv3Length           = 49
		tlv3SpecStr          = "03310266e4598d1d3c415f572a8488830b60f7e744ed9235eb0b1ba93283b315c0351800000000000000010000000000000002"
	)

	// For reuse encoding TLVs.
	var b [8]byte

	tlv1Record := makeTestRecord(t, tlv1Type, tlv1Length, tlv1SpecStr, b)
	tlv2Record := makeTestRecord(t, tlv2Type, tlv2Length, tlv2SpecStr, b)
	tlv3Record := makeTestRecord(t, tlv3Type, tlv3Length, tlv3SpecStr, b)

	// Create a signature record
	sigType := tlv.Type(signatureFieldStart)
	var sigValue uint64 = 3

	tlvSigRecord := tlv.MakePrimitiveRecord(sigType, &sigValue)

	tests := []struct {
		name   string
		tlvs   []tlv.Record
		merkle string
		err    error
	}{
		{
			name: "single tlv",
			tlvs: []tlv.Record{
				tlv1Record,
			},
			merkle: "aa0aa0f694c85492ac459c1de9831a37682985f5e840ecc9b1e28eece7dc5236",
		},
		{
			name: "two tlvs",
			tlvs: []tlv.Record{
				tlv1Record, tlv2Record,
			},
			merkle: "013b756ed73554cbc4dd3d90f363cb7cba6d8a279465a21c464e582b173ff502",
		},
		{
			name: "three tlvs",
			tlvs: []tlv.Record{
				tlv1Record, tlv2Record, tlv3Record,
			},
			merkle: "016fcda3b6f9ca30b35936877ca591fa101365a761a1453cfd9436777d593656",
		},
		{
			name: "only sig record",
			tlvs: []tlv.Record{
				tlvSigRecord,
			},
			err: ErrNoTLVs,
		},
	}

	for _, testCase := range tests {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			root, err := MerkleRoot(testCase.tlvs)
			require.True(t, errors.Is(err, testCase.err))

			// If we're expecting an error for the test case,
			// don't check the root.
			if testCase.err != nil {
				return
			}

			actual := hex.EncodeToString(root[:])
			require.Equal(t, testCase.merkle, actual)
		})
	}
}

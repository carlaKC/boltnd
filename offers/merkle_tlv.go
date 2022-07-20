package offers

import (
	"bytes"
	"fmt"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/lightningnetwork/lnd/tlv"
)

var (
	// signatureFieldStart is the beginning of the inclusive tlv range that
	// contains signatures.
	signatureFieldStart tlv.Type = 240

	// signatureFieldEnd is the end of the inclusive tlv range that contains
	// signatures.
	signatureFieldEnd tlv.Type = 1000

	// TLVTag is the tag used for nodes containing TLV values.
	TLVTag = []byte("LnLeaf")

	// NonceTag is the tag used for nodes containing TLV nonces.
	NonceTag = []byte("LnAll")
)

// tlvEncode is the function signature used to encode TLV records.
type tlvEncode func(record tlv.Record, b [8]byte) ([]byte, error)

// node is an interface implemented by nodes in our offer tlv merkle tree.
type node interface {
	// TaggedHash produces the appropriate tagged hash for the level of
	// the tree.
	TaggedHash() chainhash.Hash
}

// orderNodes takes a left and right node that can be hashed into our tree
// and returned the sorted value of their hashes for combination in the next
// level of the tree.
func orderNodes(left, right node) (chainhash.Hash, chainhash.Hash) {
	leftHash, rightHash := left.TaggedHash(), right.TaggedHash()

	// If left > right hash, then we switch the ordering so that the left
	// hash is the lesser hash.
	if bytes.Compare(leftHash[:], rightHash[:]) > 0 {
		return rightHash, leftHash
	}

	return leftHash, rightHash
}

// TLVLeaf represents a leaf in our offer merkle tree.
type TLVLeaf struct {
	// Tag is the tag to be used when hashing the value into a node.
	Tag []byte

	// Value is the value contained in the leaf.
	Value []byte
}

// Compile time assertion that TLVLeaf satisfies the node interface.
var _ node = (*TLVLeaf)(nil)

// TaggedHash computes the tagged hash of a leaf, as defined by:
// H(tag, msg) = sha256(sha256(tag) || sha256(tag) || msg)
// TaggedHash = H("LnLeaf"/"LnAll" + all_tlvs , tlv )
//
// Note: the String() function on chainhash.Hash produces a hex-encoded byte
// reversed hash (and the spec test vectors aren't reversed).
func (t *TLVLeaf) TaggedHash() chainhash.Hash {
	return *chainhash.TaggedHash(t.Tag, t.Value)
}

// CreateTLVLeaves creates a set of merkle leaves for a set of offer TLV
// records, excluding signature fields. A tlvEncode function is passed in for
// easy testing.
func CreateTLVLeaves(records []tlv.Record, encode tlvEncode) ([]*TLVLeaf,
	error) {

	// First, sort the records in canonical order.
	tlv.SortRecords(records)

	var (
		// Store each of our serialised TLVs to use for creating
		// individual leaves.
		encodedTLVs [][]byte

		// Store the concatenation of all our TLV records separately
		// to be used for nonce calculation.
		allTLVs []byte

		// Allocate for re-use encoding TLVs.
		b [8]byte
	)

	for _, record := range records {
		if isSignatureTLV(record) {
			continue
		}

		tlvBytes, err := encode(record, b)
		if err != nil {
			return nil, fmt.Errorf("encode tlv: %v, %w",
				record.Type(), err)
		}

		encodedTLVs = append(encodedTLVs, tlvBytes)
		allTLVs = append(allTLVs, tlvBytes...)
	}

	// Each TLV record leaf is paired with a nonce leaf, so we allocate
	// a slice 2x the number of our leaf records.
	leaves := make([]*TLVLeaf, len(encodedTLVs)*2)

	for i := 0; i < len(encodedTLVs); i++ {
		tlv := encodedTLVs[i]

		// We take up two elements in the leaf slice per iteration,
		// so our index is 2x the record's index.
		leafIndex := i * 2

		// Our nonce leaf follows the tlv record in our leaf slice.
		nonceIndex := leafIndex + 1

		// Create the pair of leaves for this tlv.
		leaves[leafIndex], leaves[nonceIndex] = createLeafPair(
			tlv, allTLVs,
		)
	}

	return leaves, nil
}

// createLeafPair creates a tlv and nonce pair of leaves for a TLV record.
func createLeafPair(tlv, allTLVs []byte) (*TLVLeaf, *TLVLeaf) {
	tlvLeaf := &TLVLeaf{
		Tag:   TLVTag,
		Value: tlv,
	}

	nonceLeaf := &TLVLeaf{
		Tag:   append(NonceTag, allTLVs...),
		Value: tlv,
	}

	return tlvLeaf, nonceLeaf
}

// isSignatureTLV returns a boolean indicating whether a TLV contains a
// signature.
func isSignatureTLV(record tlv.Record) bool {
	tlvType := record.Type()

	return tlvType >= signatureFieldStart &&
		tlvType <= signatureFieldEnd
}

// encodeTLV serialized a record in our typical type / length / value format.
func encodeTLV(record tlv.Record, b [8]byte) ([]byte, error) {
	w := new(bytes.Buffer)

	err := tlv.WriteVarInt(w, uint64(record.Type()), &b)
	if err != nil {
		return nil, fmt.Errorf("encode type: %w", err)
	}

	// Write the record’s length as a varint.
	err = tlv.WriteVarInt(w, record.Size(), &b)
	if err != nil {
		return nil, fmt.Errorf("encode length: %w", err)
	}

	// Encode the current record’s value.
	err = record.Encode(w)
	if err != nil {
		return nil, fmt.Errorf("encode value: %w", err)
	}

	return w.Bytes(), nil
}

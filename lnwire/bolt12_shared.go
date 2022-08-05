package lnwire

import (
	"bytes"
	"fmt"

	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/tlv"
)

// encodeFetauresRecord creates a tlv record with the type provided, encoding
// the feature vector provided as a byte vector. If the vector provided is nil
// or empty the record returned will be nil.
func encodeFetauresRecord(recordType tlv.Type,
	features *lnwire.FeatureVector) (*tlv.Record, error) {

	if features == nil {
		return nil, nil
	}

	if features.IsEmpty() {
		return nil, nil
	}

	w := new(bytes.Buffer)

	if err := features.Encode(w); err != nil {
		return nil, fmt.Errorf("encode features: %w", err)
	}

	featureBytes := w.Bytes()

	record := tlv.MakePrimitiveRecord(recordType, &featureBytes)
	return &record, nil
}

// decodeFeaturesRecord decodes the features record provided. If it is not
// present, an empty feature vector will be returned for easy use.
func decodeFeaturesRecord(decodedFeatures []byte,
	found bool) (*lnwire.FeatureVector, error) {

	// Create an empty raw feature vector.
	rawFeatures := lnwire.NewRawFeatureVector()

	// If we decoded a value for our feature vector, decode it.
	if found {
		err := rawFeatures.Decode(bytes.NewReader(decodedFeatures))
		if err != nil {
			return nil, fmt.Errorf("raw features decode: %w", err)
		}
	}
	return lnwire.NewFeatureVector(rawFeatures, lnwire.Features), nil
}

// tlvTree is an interface implemented by bolt 12 artifacts that can be
// summarized in a tlv merkle tree.
type tlvTree interface {
	// records returns a set of tlv records for all populated tlv fields.
	records() ([]tlv.Record, error)
}

// decodeMerkleRoot produces a tlv merkle tree root for a tlv stream that we
// have decoded. Since the sender may have populated the tree with odd tlv
// records that are unknown to us, we include odd unknown records that were
// parsed but not understood (by our decoding, the sender would have known
// them) in our tree.
func decodeMerkleRoot(recordProducer tlvTree,
	tlvMap map[tlv.Type][]byte) (lntypes.Hash, error) {

	populatedRecords, err := recordProducer.records()
	if err != nil {
		return lntypes.Hash{}, fmt.Errorf("get records: %w", err)
	}

	root, err := MerkleRoot(
		append(populatedRecords, unknownRecordsFromParsed(tlvMap)...),
	)
	if err != nil {
		return lntypes.Hash{}, fmt.Errorf("merkle root: %w", err)
	}

	hash, err := lntypes.MakeHash(root[:])
	if err != nil {
		return lntypes.Hash{}, fmt.Errorf("make hash: %w", err)
	}

	return hash, nil
}

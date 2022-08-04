package lnwire

import (
	"bytes"
	"fmt"

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

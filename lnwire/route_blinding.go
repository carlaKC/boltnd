package lnwire

import (
	"bytes"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/lightningnetwork/lnd/tlv"
)

const (
	// nextNodeType is a record type for the unblinded next node ID.
	nextNodeType tlv.Type = 4

	// nextBlindingOverride is a record type containing a blinding override.
	nextBlindingOverride tlv.Type = 8
)

// BlindedRouteData holds the fields that we encrypt in route blinding blobs.
type BlindedRouteData struct {
	// NextNodeID is the unblinded node id of the next hop in the route.
	NextNodeID *btcec.PublicKey

	// NextBlindingOverride is an optional blinding override used to switch
	// out ephemeral keys.
	NextBlindingOverride *btcec.PublicKey
}

// EncodeBlindedRouteData encodes a blinded route tlv stream.
func EncodeBlindedRouteData(data *BlindedRouteData) ([]byte, error) {
	w := new(bytes.Buffer)

	var records []tlv.Record

	if data.NextNodeID != nil {
		nodeIDRecord := tlv.MakePrimitiveRecord(
			nextNodeType, &data.NextNodeID,
		)
		records = append(records, nodeIDRecord)
	}

	if data.NextBlindingOverride != nil {
		overrideRecord := tlv.MakePrimitiveRecord(
			nextBlindingOverride, &data.NextBlindingOverride,
		)
		records = append(records, overrideRecord)
	}

	stream, err := tlv.NewStream(records...)
	if err != nil {
		return nil, err
	}

	if err := stream.Encode(w); err != nil {
		return nil, err
	}

	return w.Bytes(), nil
}

// DecodeBlindedRouteData decodes a blinded route tlv stream.
func DecodeBlindedRouteData(data []byte) (*BlindedRouteData, error) {
	r := bytes.NewReader(data)

	var routeData = &BlindedRouteData{}

	records := []tlv.Record{
		tlv.MakePrimitiveRecord(nextNodeType, &routeData.NextNodeID),
		tlv.MakePrimitiveRecord(
			nextBlindingOverride, &routeData.NextBlindingOverride,
		),
	}

	stream, err := tlv.NewStream(records...)
	if err != nil {
		return nil, err
	}

	if err := stream.Decode(r); err != nil {
		return nil, err
	}

	return routeData, nil
}

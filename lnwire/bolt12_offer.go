package lnwire

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/tlv"
)

const (
	// chainType is a record type setting the genesis hash of the chain
	// an offer is for.
	chainType tlv.Type = 2

	// amountType is a record type specifying the minimum amount for an
	// offer.
	amountType tlv.Type = 8

	// descriptionType is a record type for offer descriptions.
	descriptionType tlv.Type = 10

	// featuresType is a record type for the feature bits an offer requires.
	featuresType tlv.Type = 12

	// expiryType is a record type for offer expiry time.
	expiryType tlv.Type = 14

	// issuerType is a record type for identifying the issuer of an offer.
	issuerType tlv.Type = 20

	// quantityMinType is a record type that sets the minimum quantity of
	// invoices for the offer.
	quantityMinType tlv.Type = 22

	// quantityMinType is a record type that sets the maximum quanitity of
	// invoices for the offer.
	quantityMaxType tlv.Type = 24

	// nodeIDType is a record for the node's ID.
	nodeIDType tlv.Type = 30

	// signatureType is a record for a bip40 signature over the offer.
	signatureType tlv.Type = 240
)

var (
	// ErrNodeIDRequired is returned when a node pubkey is not provided
	// for an offer. Note that when blinded paths are supported, we can
	// relax this requirement.
	ErrNodeIDRequired = errors.New("node pubkey required for offer")

	// ErrQuantityRange is returned when we get an min/max quantity range
	// with min > max, which does not make sense.
	ErrQuantityRange = errors.New("invalid quantity range")

	// ErrDescriptionRequried is returned when an offer is invalid because
	//  does not contain a description.
	ErrDescriptionRequried = errors.New("offer description required")
)

// Offer represents a bolt 12 offer.
type Offer struct {
	// Chainhash is the genesis block hash of the network that the offer is
	// for.
	Chainhash lntypes.Hash

	// MinimumAmount is an optional minimum amount for the offer.
	MinimumAmount lnwire.MilliSatoshi

	// Description is an optional description of the offer.
	Description string

	// Features are the specification features that the offer requires and
	// supports.
	Features *lnwire.FeatureVector

	// Expiry is an optional expiry time of the offer.
	Expiry time.Time

	// Issuer identifies the issuing party.
	Issuer string

	// QuantityMin is the minimum number of invoices for an offer.
	QuantityMin uint64

	// QuantityMax is the maximum number of invoices for an offer.
	QuantityMax uint64

	// NodeID is the public key advertized by the offering node.
	// Note: at present this is encoded as a x-only 32 byte pubkey, but the
	// spec is set to change, so in future this should be encoded as a 33
	// byte compressed pubkey.
	NodeID *btcec.PublicKey

	// Signature is the bip340 signature for the offer.
	Signature *[64]byte

	// MerkleRoot is the merkle root of all the non-signature tlvs included
	// in the offer. This field isn't actually encoded in our tlv stream,
	// but rather calculated from it.
	MerkleRoot lntypes.Hash
}

// Compile time check that offer implements the tlvTree interface.
var _ tlvTree = (*Offer)(nil)

// records returns a set of tlv records for all of the offer's populated fields.
func (o *Offer) records() ([]tlv.Record, error) {
	var records []tlv.Record

	if o.Chainhash != lntypes.ZeroHash {
		var chainHash [32]byte
		copy(chainHash[:], o.Chainhash[:])

		record := tlv.MakePrimitiveRecord(chainType, &chainHash)
		records = append(records, record)
	}

	if o.MinimumAmount != 0 {
		amountMin := uint64(o.MinimumAmount)
		records = append(records, tu64Record(amountType, &amountMin))
	}

	if o.Description != "" {
		descriptionBytes := []byte(o.Description)

		descriptionRecord := tlv.MakePrimitiveRecord(
			descriptionType, &descriptionBytes,
		)

		records = append(records, descriptionRecord)
	}

	featuresRecord, err := encodeFetauresRecord(featuresType, o.Features)
	if err != nil {
		return nil, fmt.Errorf("encode features: %w", err)
	}

	if featuresRecord != nil {
		records = append(records, *featuresRecord)
	}

	if !o.Expiry.IsZero() {
		expirySeconds := uint64(o.Expiry.Unix())

		records = append(
			records, tu64Record(expiryType, &expirySeconds),
		)
	}

	if o.Issuer != "" {
		issuerBytes := []byte(o.Issuer)

		issuerRecord := tlv.MakePrimitiveRecord(issuerType, &issuerBytes)
		records = append(records, issuerRecord)
	}

	if o.QuantityMin != 0 {
		minRecord := tu64Record(quantityMinType, &o.QuantityMin)
		records = append(records, minRecord)
	}

	if o.QuantityMax != 0 {
		maxRecord := tu64Record(quantityMaxType, &o.QuantityMax)
		records = append(records, maxRecord)
	}

	if o.NodeID != nil {
		// Serialized as x-only pubkey.
		var nodeID [32]byte
		copy(nodeID[:], schnorr.SerializePubKey(o.NodeID))

		nodeIDRecord := tlv.MakePrimitiveRecord(
			nodeIDType, &nodeID,
		)

		records = append(records, nodeIDRecord)
	}

	if o.Signature != nil {
		sigRecord := tlv.MakePrimitiveRecord(
			signatureType, o.Signature,
		)

		records = append(records, sigRecord)
	}

	return records, nil
}

// Validate performs the validation outlined in the specification for offers.
func (o *Offer) Validate() error {
	// At present, we only support offers that contain node IDs because
	// support for blinded paths has not been added.
	//
	// The spec notes "if it sets a node ID ... otherwise MUST provide at
	// least one blinded path".
	// TODO - expand validation once blinded paths are added.
	if o.NodeID == nil {
		return ErrNodeIDRequired
	}

	if o.Description == "" {
		return ErrDescriptionRequried
	}

	var (
		minQuantitySet = o.QuantityMin != 0
		maxQuantitySet = o.QuantityMax != 0
	)

	// If we have values for both, enforce max > min.
	if minQuantitySet && maxQuantitySet && o.QuantityMin > o.QuantityMax {
		return fmt.Errorf("min: %v > max: %v quantity, %w",
			o.QuantityMin, o.QuantityMax, ErrQuantityRange)
	}

	// Check that our signature is a valid signature of the merkle root for
	// the offer.
	if o.Signature != nil {
		sigDigest := signatureDigest(
			offerTag, signatureTag, o.MerkleRoot,
		)

		if err := validateSignature(
			*o.Signature, o.NodeID, sigDigest[:],
		); err != nil {
			return err
		}
	}

	return nil
}

// EncodeOffer encodes an offer.
func EncodeOffer(offer *Offer) ([]byte, error) {
	records, err := offer.records()
	if err != nil {
		return nil, fmt.Errorf("get records: %w", err)
	}

	stream, err := tlv.NewStream(records...)
	if err != nil {
		return nil, fmt.Errorf("offer encode stream: %w", err)
	}

	buf := new(bytes.Buffer)
	if err := stream.Encode(buf); err != nil {
		return nil, fmt.Errorf("offer encode tlvs: %w", err)
	}

	return buf.Bytes(), nil
}

// DecodeOffer decodes a bolt 12 offer TLV stream.
func DecodeOffer(offerBytes []byte) (*Offer, error) {
	offer := &Offer{}

	var (
		amountMin                     uint64
		expirySeconds                 uint64
		features, description, issuer []byte
		chainHash, nodeID             [32]byte
		signature                     [64]byte
	)

	records := []tlv.Record{
		tlv.MakePrimitiveRecord(chainType, &chainHash),
		tu64Record(amountType, &amountMin),
		tlv.MakePrimitiveRecord(descriptionType, &description),
		tlv.MakePrimitiveRecord(featuresType, &features),
		tu64Record(expiryType, &expirySeconds),
		tlv.MakePrimitiveRecord(issuerType, &issuer),
		tu64Record(quantityMinType, &offer.QuantityMin),
		tu64Record(quantityMaxType, &offer.QuantityMax),
		tlv.MakePrimitiveRecord(nodeIDType, &nodeID),
		tlv.MakePrimitiveRecord(signatureType, &signature),
	}

	stream, err := tlv.NewStream(records...)
	if err != nil {
		return nil, fmt.Errorf("offer decode stream: %w", err)
	}

	r := bytes.NewReader(offerBytes)
	tlvMap, err := stream.DecodeWithParsedTypes(r)
	if err != nil {
		return nil, fmt.Errorf("offer decode: %w", err)
	}

	// Add typed values to our offer that were decoded using intermediate
	// vars.
	if _, ok := tlvMap[chainType]; ok {
		offer.Chainhash, err = lntypes.MakeHash(chainHash[:])
		if err != nil {
			return nil, fmt.Errorf("chain hash: %w", err)
		}
	}

	if _, ok := tlvMap[amountType]; ok {
		offer.MinimumAmount = lnwire.MilliSatoshi(amountMin)
	}

	if _, ok := tlvMap[expiryType]; ok {
		offer.Expiry = time.Unix(int64(expirySeconds), 0)
	}

	// We want to set a non-nil (empty) feature vector for our offer even
	// if no TLV was set, so we optionally decode the feature vector if it
	// was provided, setting an empty vector if it was not.
	_, found := tlvMap[featuresType]
	offer.Features, err = decodeFeaturesRecord(features, found)
	if err != nil {
		return nil, fmt.Errorf("decode features: %w", err)
	}

	if _, ok := tlvMap[descriptionType]; ok {
		offer.Description = string(description)
	}

	if _, ok := tlvMap[issuerType]; ok {
		offer.Issuer = string(issuer)
	}

	if _, ok := tlvMap[nodeIDType]; ok {
		// Parse x-only pubkey from raw bytes.
		pubkey, err := schnorr.ParsePubKey(nodeID[:])
		if err != nil {
			return nil, fmt.Errorf("invalid pubkey: %w", err)
		}

		offer.NodeID = pubkey
	}

	if _, ok := tlvMap[signatureType]; ok {
		offer.Signature = &signature
	}

	offer.MerkleRoot, err = decodeMerkleRoot(offer, tlvMap)
	if err != nil {
		return nil, fmt.Errorf("merkle root: %w", err)
	}

	return offer, nil
}

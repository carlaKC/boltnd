package lnwire

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/lightningnetwork/lnd/lntypes"
	lndwire "github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/tlv"
)

const (
	// invReqOfferIDType is a record containing the associated offer ID.
	invReqOfferIDType tlv.Type = 4

	// invReqAmountType is a record containing the invoice amount
	// requested.
	invReqAmountType tlv.Type = 8

	// invReqFeaturesType is a record containing the feature vector for
	// the request.
	invReqFeaturesType tlv.Type = 12

	// invReqQuantityType is a record for the quantity requested.
	invReqQuantityType tlv.Type = 32

	// invReqPayerKeyType is a record for the "proof-of-payer" key provided
	// by the sender.
	invReqPayerKeyType tlv.Type = 38

	// invReqPayerNoteType is a record for a note from the sender.
	invReqPayerNoteType tlv.Type = 39

	// invReqPayerInfoType is a record for arbitrary information from the
	// sender.
	invReqPayerInfoType tlv.Type = 50

	// invReqSignatureType is a record for the signature of the request's
	// merkle root.
	invReqSignatureType tlv.Type = 240
)

var (
	// ErrOfferIDRequired is returned when an invoice request does not
	// contain an associated offer id.
	ErrOfferIDRequired = errors.New("offer id required")

	// ErrPayerKeyRequired is returned when an invoice request does not
	// contain a payer key.
	ErrPayerKeyRequired = errors.New("payer key required")

	// ErrSignatureRequired is returned when an invoice request does not
	// contain a signature.
	ErrSignatureRequired = errors.New("signature required")
)

// InvoiceRequest represents a bolt 12 request for an invoice.
type InvoiceRequest struct {
	// OfferID is the merkle root of the offer this request is associated
	// with.
	OfferID lntypes.Hash

	// Amount is the invoice amount that the request is for.
	Amount lndwire.MilliSatoshi

	// Features is the set of features required for the invoice.
	Features *lndwire.FeatureVector

	// Quantity is the number of items that the invoice is for.
	Quantity uint64

	// PayerKey is a proof-of-payee key for the sender.
	PayerKey *btcec.PublicKey

	// PayerNote is a note from the sender.
	PayerNote string

	// PayerInfo is arbitrary information included by the sender.
	PayerInfo []byte

	// Signature is an optional signature on the tlv merkle root of the
	// request.
	Signature *[64]byte

	// MerkleRoot is the merkle root of all the non-signature tlvs included
	// in the invoice request. This field isn't actually encoded in our
	// tlv stream, but rather calculated from it.
	MerkleRoot lntypes.Hash
}

// Compile time check that invoice request implements the tlv tree interface.
var _ tlvTree = (*InvoiceRequest)(nil)

// Validate performs validation on an invoice request as described in the
// specification.
func (i *InvoiceRequest) Validate() error {
	if i.OfferID == lntypes.ZeroHash {
		return ErrOfferIDRequired
	}

	if i.PayerKey == nil {
		return ErrPayerKeyRequired
	}

	if i.Signature == nil {
		return ErrSignatureRequired
	}

	// Check that our signature is a valid signature of the merkle root for
	// the request.
	sigDigest := signatureDigest(
		invoiceRequestTag, signatureTag, i.MerkleRoot,
	)

	if err := validateSignature(
		*i.Signature, i.PayerKey, sigDigest[:],
	); err != nil {
		return err
	}

	return nil
}

// records returns a set of records for all the non-nil fields in an invoice
// request.
func (i *InvoiceRequest) records() ([]tlv.Record, error) {
	var records []tlv.Record

	if i.OfferID != lntypes.ZeroHash {
		var offerID [32]byte
		copy(offerID[:], i.OfferID[:])

		record := tlv.MakePrimitiveRecord(invReqOfferIDType, &offerID)
		records = append(records, record)
	}

	if i.Amount != 0 {
		amount := uint64(i.Amount)

		record := tlv.MakePrimitiveRecord(invReqAmountType, &amount)
		records = append(records, record)
	}

	featuresRecord, err := encodeFetauresRecord(
		invReqFeaturesType, i.Features,
	)
	if err != nil {
		return nil, fmt.Errorf("encode features: %w", err)
	}

	if featuresRecord != nil {
		records = append(records, *featuresRecord)
	}

	if i.Quantity != 0 {
		record := tlv.MakePrimitiveRecord(
			invReqQuantityType, &i.Quantity,
		)
		records = append(records, record)
	}

	if i.PayerKey != nil {
		record := tlv.MakePrimitiveRecord(
			invReqPayerKeyType, &i.PayerKey,
		)
		records = append(records, record)
	}

	if i.PayerNote != "" {
		note := []byte(i.PayerNote)

		record := tlv.MakePrimitiveRecord(invReqPayerNoteType, &note)
		records = append(records, record)
	}

	if len(i.PayerInfo) != 0 {
		record := tlv.MakePrimitiveRecord(
			invReqPayerInfoType, &i.PayerInfo,
		)
		records = append(records, record)
	}

	if i.Signature != nil {
		signature := *i.Signature

		record := tlv.MakePrimitiveRecord(
			invReqSignatureType, &signature,
		)
		records = append(records, record)
	}

	return records, nil
}

// EncodeInvoiceRequest encodes a bolt12 invoice request as a tlv stream.
func EncodeInvoiceRequest(i *InvoiceRequest) ([]byte, error) {
	records, err := i.records()
	if err != nil {
		return nil, fmt.Errorf("%w: invoice request records", err)
	}

	stream, err := tlv.NewStream(records...)
	if err != nil {
		return nil, fmt.Errorf("new stream: %w", err)
	}

	b := new(bytes.Buffer)
	if err := stream.Encode(b); err != nil {
		return nil, fmt.Errorf("encode stream: %w", err)
	}

	return b.Bytes(), nil
}

// DecodeInvoiceRequest decodes a bolt12 invoice request tlv stream.
func DecodeInvoiceRequest(b []byte) (*InvoiceRequest, error) {
	var (
		i                   = &InvoiceRequest{}
		offerID             [32]byte
		amount              uint64
		features, payerNote []byte
		signature           [64]byte
	)

	records := []tlv.Record{
		tlv.MakePrimitiveRecord(invReqOfferIDType, &offerID),
		tlv.MakePrimitiveRecord(invReqAmountType, &amount),
		tlv.MakePrimitiveRecord(invReqFeaturesType, &features),
		tlv.MakePrimitiveRecord(invReqQuantityType, &i.Quantity),
		tlv.MakePrimitiveRecord(invReqPayerKeyType, &i.PayerKey),
		tlv.MakePrimitiveRecord(invReqPayerNoteType, &payerNote),
		tlv.MakePrimitiveRecord(invReqPayerInfoType, &i.PayerInfo),
		tlv.MakePrimitiveRecord(invReqSignatureType, &signature),
	}

	stream, err := tlv.NewStream(records...)
	if err != nil {
		return nil, fmt.Errorf("new stream: %w", err)
	}

	r := bytes.NewReader(b)
	tlvMap, err := stream.DecodeWithParsedTypes(r)
	if err != nil {
		return nil, fmt.Errorf("decode stream: %w", err)
	}

	if _, ok := tlvMap[invReqOfferIDType]; ok {
		i.OfferID, err = lntypes.MakeHash(offerID[:])
		if err != nil {
			return nil, fmt.Errorf("offer id: %w", err)
		}
	}

	if _, ok := tlvMap[invReqAmountType]; ok {
		i.Amount = lndwire.MilliSatoshi(amount)
	}

	_, found := tlvMap[invReqFeaturesType]
	i.Features, err = decodeFeaturesRecord(features, found)
	if err != nil {
		return nil, fmt.Errorf("decode features: %w", err)
	}

	if _, ok := tlvMap[invReqPayerNoteType]; ok {
		i.PayerNote = string(payerNote)
	}

	if _, ok := tlvMap[invReqSignatureType]; ok {
		i.Signature = &signature
	}

	i.MerkleRoot, err = decodeMerkleRoot(i, tlvMap)
	if err != nil {
		return nil, fmt.Errorf("merkle root: %w", err)
	}

	return i, nil
}

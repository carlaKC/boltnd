package lnwire

import (
	"bytes"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/lightningnetwork/lnd/lnwire"
	lndwire "github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/tlv"
)

const (
	invReqOfferIDType   tlv.Type = 4
	invReqAmountType    tlv.Type = 8
	invReqFeaturesType  tlv.Type = 12
	invReqQuantityType  tlv.Type = 32
	invReqPayerKeyType  tlv.Type = 38
	invReqPayerNoteType tlv.Type = 39
	invReqPayerInfoType tlv.Type = 50
	invReqSignatureType tlv.Type = 240
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

	if i.Features != nil && !i.Features.IsEmpty() {
		w := new(bytes.Buffer)

		if err := i.Features.Encode(w); err != nil {
			return nil, fmt.Errorf("encode features: %w", err)
		}

		features := w.Bytes()

		record := tlv.MakePrimitiveRecord(
			invReqFeaturesType, &features,
		)

		records = append(records, record)
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

	rawFeatures := lnwire.NewRawFeatureVector()
	if _, ok := tlvMap[invReqFeaturesType]; ok {
		err := rawFeatures.Decode(bytes.NewReader(features))
		if err != nil {
			return nil, fmt.Errorf("raw features decode: %w", err)
		}
	}
	i.Features = lnwire.NewFeatureVector(rawFeatures, lnwire.Features)

	if _, ok := tlvMap[invReqPayerNoteType]; ok {
		i.PayerNote = string(payerNote)
	}

	if _, ok := tlvMap[invReqSignatureType]; ok {
		i.Signature = &signature
	}

	return i, nil
}

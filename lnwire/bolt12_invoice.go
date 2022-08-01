package lnwire

import (
	"bytes"
	"fmt"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/lightningnetwork/lnd/lnwire"
	lndwire "github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/tlv"
)

const (
	// offerIDType is a record holding the offer ID that an invoice is
	// associated with.
	offerIDType tlv.Type = 4

	// invAmountType is a record for the amount for an invoice.
	invAmountType tlv.Type = 8

	// invDescType is a record for a description for the invoice.
	invDescType tlv.Type = 10

	// invFeatType is a record containing the features required for the
	// invoice.
	invFeatType tlv.Type = 12

	// invNodeIDType is a record for the node's public key.
	invNodeIDType tlv.Type = 30

	// invQuantityType is a record denoting the quantity that an invoice is
	// for.
	invQuantityType tlv.Type = 32

	// payerKeyType is a record containing the paying party's "proof of
	// payee" key.
	payerKeyType tlv.Type = 38

	// payerNoteType is a record for an optional payer note.
	payerNoteType tlv.Type = 39

	// createdAtType is a record for the unix timestamp that the invoice
	// was created at.
	createdAtType tlv.Type = 40

	// paymentHashType is a record for the payment hash for the invoice.
	paymentHashType tlv.Type = 42

	// relativeExpiryType is a record for the relative expiry from the
	// creation time, expressed in seconds.
	relativeExpiryType tlv.Type = 44

	// cltvType is a record for the minimum final cltv expiry of the
	// invoice.
	cltvType tlv.Type = 46

	// payerInfoType is a record for arbitrary payer information.
	payerInfoType tlv.Type = 50

	// invSigType is a record for a signature over the invoice.
	invSigType tlv.Type = 240
)

// Invoice represents a bolt 12 invoice.
type Invoice struct {
	// OfferID is the merkle root of the offer this invoice is associated
	// with.
	OfferID lntypes.Hash

	// Amount is the amount that the invoice is for.
	Amount lndwire.MilliSatoshi

	// Description is an optional description of the invoice.
	Description string

	// Features is the set of features the invoice requires.
	Features *lndwire.FeatureVector

	// NodeID is the node ID for the recipient.
	NodeID *btcec.PublicKey

	// Quantity is an optional quantity of items for the invoice.
	Quantity uint64

	// PayerKey is the paying party's "proof of payee" key.
	PayerKey *btcec.PublicKey

	// PayerNote is an optional note from the paying party.
	PayerNote string

	// CreatedAt is the time the invoice was created.
	CreatedAt time.Time

	// PaymentHash is the payment hash for this invoice.
	PaymentHash lntypes.Hash

	// RelativeExpiry is the relative expiry in seconds from the invoice's
	// created time.
	RelativeExpiry time.Duration

	// CLTVExpiry is the minimum final cltv expiry for the invoice.
	CLTVExpiry uint64

	// PayerInfo is an arbitrary piece of data set by the payer.
	PayerInfo []byte

	// Signature is a signature over the tlv merkle root of the invoice's
	// fields.
	Signature *[64]byte
}

// records returns a set of tlv records for all the non-nil invoice fields.
func (i *Invoice) records() ([]tlv.Record, error) {
	var records []tlv.Record

	if i.OfferID != lntypes.ZeroHash {
		var offerID [32]byte
		copy(offerID[:], i.OfferID[:])

		record := tlv.MakePrimitiveRecord(offerIDType, &offerID)
		records = append(records, record)
	}

	if i.Amount != 0 {
		amount := uint64(i.Amount)

		record := tlv.MakePrimitiveRecord(invAmountType, &amount)
		records = append(records, record)
	}

	if i.Description != "" {
		description := []byte(i.Description)

		record := tlv.MakePrimitiveRecord(
			invDescType, &description,
		)
		records = append(records, record)
	}

	if i.Features != nil && !i.Features.IsEmpty() {
		w := new(bytes.Buffer)

		if err := i.Features.Encode(w); err != nil {
			return nil, fmt.Errorf("encode features: %w", err)
		}

		features := w.Bytes()

		record := tlv.MakePrimitiveRecord(
			invFeatType, &features,
		)

		records = append(records, record)
	}

	if i.NodeID != nil {
		record := tlv.MakePrimitiveRecord(invNodeIDType, &i.NodeID)
		records = append(records, record)
	}

	if i.Quantity != 0 {
		record := tlv.MakePrimitiveRecord(invQuantityType, &i.Quantity)
		records = append(records, record)
	}

	if i.PayerKey != nil {
		record := tlv.MakePrimitiveRecord(payerKeyType, &i.PayerKey)
		records = append(records, record)
	}

	if i.PayerNote != "" {
		note := []byte(i.PayerNote)

		record := tlv.MakePrimitiveRecord(invNodeIDType, &note)
		records = append(records, record)
	}

	if !i.CreatedAt.IsZero() {
		created := i.CreatedAt.Unix()

		record := tlv.MakePrimitiveRecord(createdAtType, &created)
		records = append(records, record)
	}

	if i.PaymentHash != lntypes.ZeroHash {
		var payHash [32]byte
		copy(payHash[:], i.PaymentHash[:])

		record := tlv.MakePrimitiveRecord(paymentHashType, &payHash)
		records = append(records, record)
	}

	if i.RelativeExpiry != 0 {
		expiry := uint64(i.RelativeExpiry.Seconds())

		record := tlv.MakePrimitiveRecord(relativeExpiryType, &expiry)
		records = append(records, record)
	}

	if i.CLTVExpiry != 0 {
		record := tlv.MakePrimitiveRecord(cltvType, &i.CLTVExpiry)
		records = append(records, record)
	}

	if len(i.PayerInfo) != 0 {
		record := tlv.MakePrimitiveRecord(payerInfoType, &i.PayerInfo)
		records = append(records, record)
	}

	if i.Signature != nil {
		signature := *i.Signature

		record := tlv.MakePrimitiveRecord(invSigType, &signature)
		records = append(records, record)
	}

	return records, nil
}

// EncodeInvoice encodes a bolt12 invoice as a tlv stream.
func EncodeInvoice(i *Invoice) ([]byte, error) {
	records, err := i.records()
	if err != nil {
		return nil, fmt.Errorf("%w: invoice records", err)
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

// DecodeInvoice decodes a bolt12 invoice tlv stream.
func DecodeInvoice(b []byte) (*Invoice, error) {
	var (
		i                                = &Invoice{}
		offerID, payHash                 [32]byte
		amount                           uint64
		features, description, payerNote []byte
		createdAt                        uint64
		relativeExpiry                   uint64
		signature                        [64]byte
	)
	records := []tlv.Record{
		tlv.MakePrimitiveRecord(offerIDType, &offerID),
		tlv.MakePrimitiveRecord(invAmountType, &amount),
		tlv.MakePrimitiveRecord(invDescType, &description),
		tlv.MakePrimitiveRecord(invFeatType, &features),
		tlv.MakePrimitiveRecord(invNodeIDType, &i.NodeID),
		tlv.MakePrimitiveRecord(invQuantityType, &i.Quantity),
		tlv.MakePrimitiveRecord(payerKeyType, &i.PayerKey),
		tlv.MakePrimitiveRecord(invNodeIDType, &payerNote),
		tlv.MakePrimitiveRecord(createdAtType, &createdAt),
		tlv.MakePrimitiveRecord(paymentHashType, &payHash),
		tlv.MakePrimitiveRecord(relativeExpiryType, &relativeExpiry),
		tlv.MakePrimitiveRecord(cltvType, &i.CLTVExpiry),
		tlv.MakePrimitiveRecord(payerInfoType, &i.PayerInfo),
		tlv.MakePrimitiveRecord(invSigType, &signature),
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

	if _, ok := tlvMap[offerIDType]; ok {
		i.OfferID, err = lntypes.MakeHash(offerID[:])
		if err != nil {
			return nil, fmt.Errorf("offer id: %w", err)
		}
	}

	if _, ok := tlvMap[invAmountType]; ok {
		i.Amount = lndwire.MilliSatoshi(amount)
	}

	rawFeatures := lnwire.NewRawFeatureVector()
	if _, ok := tlvMap[invFeatType]; ok {
		err := rawFeatures.Decode(bytes.NewReader(features))
		if err != nil {
			return nil, fmt.Errorf("raw features decode: %w", err)
		}
	}
	i.Features = lnwire.NewFeatureVector(rawFeatures, lnwire.Features)

	if _, ok := tlvMap[invDescType]; ok {
		i.Description = string(description)
	}

	if _, ok := tlvMap[payerNoteType]; ok {
		i.PayerNote = string(payerNote)
	}

	if _, ok := tlvMap[createdAtType]; ok {
		i.CreatedAt = time.Unix(int64(createdAt), 0)
	}

	if _, ok := tlvMap[paymentHashType]; ok {
		i.PaymentHash, err = lntypes.MakeHash(payHash[:])
		if err != nil {
			return nil, fmt.Errorf("pay hash: %w", err)
		}
	}

	if _, ok := tlvMap[relativeExpiryType]; ok {
		i.RelativeExpiry = time.Second * time.Duration(relativeExpiry)
	}

	if _, ok := tlvMap[invSigType]; ok {
		i.Signature = &signature
	}

	return i, nil
}

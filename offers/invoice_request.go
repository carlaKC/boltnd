package offers

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/tlv"
)

const (
	// offerIDType is a record type containing the offer id for an invoice
	// request.
	offerIDType tlv.Type = 4

	// amountType is a record type containing the amount that an invoice
	// request is for.
	amountType tlv.Type = 6

	// featuresType is a record type indicating the features required for
	// an invoice request.
	// TODO - check meaning of this field?
	featuresType tlv.Type = 12

	// quantityType is a record type indicating the number of items the
	// invoice is requested for.
	quantityType tlv.Type = 32

	// payerKeyType is a record type containing the paying party's secret
	// key.
	payerKeyType tlv.Type = 38

	// payerNoteType is a record type containing a note from the payer.
	payerNoteType tlv.Type = 39

	// payerInfoType is a record type containing payer information.
	payerInfoType tlv.Type = 50

	// signatureType is a record type containing signatures for invoice
	// requests.
	signatureType tlv.Type = 240
)

var (
	// ErrOfferIDRequired is returned when an invoice request does not have
	// an offer ID, which is required.
	ErrOfferIDRequired = errors.New("offer ID required")

	// ErrPayerKeyRequired is returned when an invoice request does not have
	// the required payer key TLV set.
	ErrPayerKeyRequired = errors.New("payer key required")

	// ErrSignatureRequired is returned when an invoice request is missing
	// a signature TLV.
	ErrSignatureRequired = errors.New("signature required")

	// ErrInvalidRequestSig is returned when an invoice request signature
	// is invalid.
	ErrInvalidRequestSig = errors.New("invalid signature")
)

// InvoiceRequest represents a request for an invoice associated with an offer.
type InvoiceRequest struct {
	// OfferID is the merkle root of the offer that this invoice request
	// is for.
	OfferID *chainhash.Hash

	// Amount is the amount that the request is for.
	Amount lnwire.MilliSatoshi

	// Features is the set of features required in the invoice.
	Features *lnwire.FeatureVector

	// Quantity is the amount of items the requested invoice is for.
	Quantity uint64

	// PayerKey is a secret used to provide the payer with proof of payee.
	PayerKey *btcec.PublicKey

	// PayerNote is an optional note from the paying party.
	PayerNote string

	// PayerInfo contains optional additional information from the payer.
	PayerInfo []byte

	// Signature contains a signature over the tlv merkle root of the
	// request, using the payer key provided.
	Signature *[64]byte

	// MerkleRoot is the merkle root of the non-signature tlvs in the
	// invoice request.
	MerkleRoot chainhash.Hash
}

// Validate performs validation as described in the specification for an offer.
func (i *InvoiceRequest) Validate() error {
	if i.OfferID == nil {
		return ErrOfferIDRequired
	}

	if i.PayerKey == nil {
		return ErrPayerKeyRequired
	}

	if i.Signature == nil {
		return ErrSignatureRequired
	}

	// We know that our signature is non-nil, so we can validate it against
	// our merkle root.
	sig, err := schnorr.ParseSignature(i.Signature[:])
	if err != nil {
		return fmt.Errorf("invalid signature: %v: %w", *i.Signature,
			err)
	}

	if !sig.Verify(i.MerkleRoot[:], i.PayerKey) {
		return fmt.Errorf("%w: %v for: %v", ErrInvalidRequestSig,
			i.Signature[:], i.MerkleRoot)
	}

	return nil
}

// EncodeInvoiceRequest encodes an invoice request as a tlv stream.
func EncodeInvoiceRequest(request *InvoiceRequest) ([]byte, error) {
	var records []tlv.Record

	if request.OfferID != nil {
		var offerID [32]byte
		copy(offerID[:], request.OfferID[:])

		record := tlv.MakePrimitiveRecord(offerIDType, &offerID)
		records = append(records, record)
	}

	if request.Amount != 0 {
		amount := uint64(request.Amount)

		record := tlv.MakePrimitiveRecord(amountType, &amount)
		records = append(records, record)
	}

	// Encode features if present and non-empty.
	if request.Features != nil && !request.Features.IsEmpty() {
		w := new(bytes.Buffer)

		if err := request.Features.Encode(w); err != nil {
			return nil, fmt.Errorf("encode features: %w", err)
		}

		features := w.Bytes()
		record := tlv.MakePrimitiveRecord(featuresType, &features)
		records = append(records, record)
	}

	if request.Quantity != 0 {
		record := tlv.MakePrimitiveRecord(
			quantityType, &request.Quantity,
		)
		records = append(records, record)
	}

	if request.PayerKey != nil {
		record := tlv.MakePrimitiveRecord(
			payerKeyType, &request.PayerKey,
		)

		records = append(records, record)
	}

	if request.PayerNote != "" {
		note := []byte(request.PayerNote)

		record := tlv.MakePrimitiveRecord(payerNoteType, &note)
		records = append(records, record)
	}

	if len(request.PayerInfo) != 0 {
		record := tlv.MakePrimitiveRecord(
			payerInfoType, &request.PayerInfo,
		)
		records = append(records, record)
	}

	if request.Signature != nil {
		record := tlv.MakePrimitiveRecord(
			signatureType, request.Signature,
		)
		records = append(records, record)
	}

	stream, err := tlv.NewStream(records...)
	if err != nil {
		return nil, fmt.Errorf("new stream: %w", err)
	}

	w := new(bytes.Buffer)
	if err := stream.Encode(w); err != nil {
		return nil, fmt.Errorf("encode stream: %w", err)
	}

	return w.Bytes(), nil
}

// DecodeInvoiceRequest decodes an invoice request tlv stream.
func DecodeInvoiceRequest(request []byte) (*InvoiceRequest, error) {
	var (
		invRequest = &InvoiceRequest{}

		offerID             [32]byte
		amount              uint64
		features, payerNote []byte
		signature           [64]byte
	)

	records := []tlv.Record{
		tlv.MakePrimitiveRecord(offerIDType, &offerID),
		tlv.MakePrimitiveRecord(amountType, &amount),
		tlv.MakePrimitiveRecord(featuresType, &features),
		tlv.MakePrimitiveRecord(quantityType, &invRequest.Quantity),
		tlv.MakePrimitiveRecord(payerKeyType, &invRequest.PayerKey),
		tlv.MakePrimitiveRecord(payerNoteType, &payerNote),
		tlv.MakePrimitiveRecord(payerInfoType, &invRequest.PayerInfo),
		tlv.MakePrimitiveRecord(signatureType, &signature),
	}

	stream, err := tlv.NewStream(records...)
	if err != nil {
		return nil, fmt.Errorf("new stream: %w", err)
	}

	r := bytes.NewReader(request)
	tlvMap, err := stream.DecodeWithParsedTypes(r)
	if err != nil {
		return nil, fmt.Errorf("decode stream: %w", err)
	}

	if _, ok := tlvMap[offerIDType]; ok {
		invRequest.OfferID, err = chainhash.NewHash(offerID[:])
		if err != nil {
			return nil, fmt.Errorf("offer ID: %w", err)
		}
	}

	if _, ok := tlvMap[amountType]; ok {
		invRequest.Amount = lnwire.MilliSatoshi(amount)
	}

	// We always want a non-nil features vector for easy use.
	rawFeatures := lnwire.NewRawFeatureVector()
	if _, ok := tlvMap[featuresType]; ok {
		err := rawFeatures.Decode(bytes.NewReader(features))
		if err != nil {
			return nil, fmt.Errorf("raw features decode: %w", err)
		}
	}
	invRequest.Features = lnwire.NewFeatureVector(
		rawFeatures, lnwire.Features,
	)

	if _, ok := tlvMap[payerNoteType]; ok {
		invRequest.PayerNote = string(payerNote)
	}

	if _, ok := tlvMap[signatureType]; ok {
		invRequest.Signature = &signature
	}

	// Calculate merkle root with _all_ tlv types, even the odd ones we
	// didn't recognize, because the sender would have included them in
	// root calculation.
	root, err := MerkleRoot(
		recordsFromParsedTypes(tlvMap),
	)
	if err != nil {
		return nil, fmt.Errorf("merkle root: %w", err)
	}

	invRequest.MerkleRoot = *root

	return invRequest, nil
}

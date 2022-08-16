package lnwire

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/lightningnetwork/lnd/lnwire"
	lndwire "github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/tlv"
)

const (
	// invReqChainType is a record containing the chain hash for the invoice
	// request.
	invReqChainType tlv.Type = 2

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

	// ErrBelowMinAmount is returned if an attempt to make a request for
	// less than the offer minimum is created.
	ErrBelowMinAmount = errors.New("amount less than offer minimum")

	// ErrOutsideQuantityRange is returned if a quantity outside of the
	// offer's range is requested.
	ErrOutsideQuantityRange = errors.New("quantity outside range")

	// ErrNoQuantity is returned when an invoice request for a specific
	// quantity is requested from an offer with no quantity tlvs specified.
	ErrNoQuantity = errors.New("quantity requested for offer with no " +
		"quantity")

	// ErrQuantityRequired is returned when an invoice request does not
	// specify quantity for an offer that provides a quantity range.
	ErrQuantityRequired = errors.New("quantity set in offer but not " +
		"request")
)

// InvoiceRequest represents a bolt 12 request for an invoice.
type InvoiceRequest struct {
	// Chainhash is the hash of the genesis block of the chain that the
	// invoice request is for.
	Chainhash lntypes.Hash

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

// NewInvoiceRequest returns a new invoice request for the offer provided. This
// function does not produce a signature for the invoice, but it does calculate
// its tlv merkle root.
func NewInvoiceRequest(offer *Offer, amount lnwire.MilliSatoshi,
	quantity uint64, payerKey *btcec.PublicKey, payerNote string) (
	*InvoiceRequest, error) {

	if amount < offer.MinimumAmount {
		return nil, fmt.Errorf("%w: %v < %v", ErrBelowMinAmount, amount,
			offer.MinimumAmount)
	}

	var (
		haveMinQuantity = offer.QuantityMin != 0
		haveMaxQuantity = offer.QuantityMax != 0

		quantitySpecified = haveMinQuantity && haveMaxQuantity
	)

	// Fail if the offer has a quantity range but the request does not set
	// quantity>0.
	if quantitySpecified && quantity == 0 {
		return nil, fmt.Errorf("%w: %v", ErrQuantityRequired, quantity)
	}

	// Fail if we're outside of the quantity range.
	if haveMinQuantity && quantity < offer.QuantityMin {
		return nil, fmt.Errorf("%w: %v < %v", ErrOutsideQuantityRange,
			quantity, offer.QuantityMin)
	}

	if haveMaxQuantity && quantity > offer.QuantityMax {
		return nil, fmt.Errorf("%w: %v > %v", ErrOutsideQuantityRange,
			quantity, offer.QuantityMax)
	}

	// Fail if the offer does not have a quantity range but the request
	// sets one.
	if !quantitySpecified && quantity != 0 {
		return nil, fmt.Errorf("%w: %v", ErrNoQuantity, quantity)
	}

	request := &InvoiceRequest{
		OfferID:   offer.MerkleRoot,
		Amount:    amount,
		Features:  offer.Features,
		Quantity:  quantity,
		PayerKey:  payerKey,
		PayerNote: payerNote,
	}

	records, err := request.records()
	if err != nil {
		return nil, fmt.Errorf("records: %w", err)
	}

	request.MerkleRoot, err = MerkleRoot(records)
	if err != nil {
		return nil, fmt.Errorf("merkle root: %v", err)
	}

	return request, nil
}

// SignatureDigest returns the tagged digest that is signed for invoice
// requests.
func (i *InvoiceRequest) SignatureDigest() chainhash.Hash {
	return signatureDigest(invoiceRequestTag, signatureTag, i.MerkleRoot)
}

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

	if i.Chainhash != lntypes.ZeroHash {
		var chainhash [32]byte
		copy(chainhash[:], i.Chainhash[:])

		record := tlv.MakePrimitiveRecord(invReqChainType, &chainhash)
		records = append(records, record)
	}

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
		chainHash, offerID  [32]byte
		amount              uint64
		features, payerNote []byte
		signature           [64]byte
	)

	records := []tlv.Record{
		tlv.MakePrimitiveRecord(invReqChainType, &chainHash),
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

	if _, ok := tlvMap[invReqChainType]; ok {
		i.Chainhash, err = lntypes.MakeHash(chainHash[:])
		if err != nil {
			return nil, fmt.Errorf("chain hash: %w", err)
		}
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

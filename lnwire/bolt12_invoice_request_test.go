package lnwire

import (
	"errors"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/carlakc/boltnd/testutils"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/stretchr/testify/require"
)

// TestInvoiceRequestEncoding tests encoding and decoding of bolt 12 invoice
// requests. Test cases generally contain a single field per case to implicitly
// test the case where a field is not included.
func TestInvoiceRequestEncoding(t *testing.T) {
	var (
		pubkey = testutils.GetPubkeys(t, 1)[0]

		hash lntypes.Hash

		sig [64]byte
	)

	copy(hash[:], []byte{1, 2, 3})
	copy(sig[:], []byte{4, 5, 6})

	tests := []struct {
		name    string
		encoded *InvoiceRequest
	}{
		{
			name: "offer id",
			encoded: &InvoiceRequest{
				OfferID: hash,
			},
		},
		{
			name: "amount",
			encoded: &InvoiceRequest{
				Amount: lnwire.MilliSatoshi(1),
			},
		},
		{
			name: "features - empty",
			encoded: &InvoiceRequest{
				// Include a non-empty record so that our merkle
				// tree can be calculated on decode.
				PayerKey: pubkey,
				Features: lnwire.NewFeatureVector(
					lnwire.NewRawFeatureVector(),
					lnwire.Features,
				),
			},
		},
		{
			name: "features - populated",
			encoded: &InvoiceRequest{
				Features: lnwire.NewFeatureVector(
					lnwire.NewRawFeatureVector(
						lnwire.AMPOptional,
					),
					lnwire.Features,
				),
			},
		},
		{
			name: "quantity",
			encoded: &InvoiceRequest{
				Quantity: 3,
			},
		},
		{
			name: "payer key",
			encoded: &InvoiceRequest{
				PayerKey: pubkey,
			},
		},
		{
			name: "payer note",
			encoded: &InvoiceRequest{
				PayerNote: "note",
			},
		},
		{
			name: "payer info",
			encoded: &InvoiceRequest{
				PayerInfo: []byte{1, 2, 3},
			},
		},
		{
			name: "signature",
			encoded: &InvoiceRequest{
				// Include a non-sig record so that our merkle
				// tree can be calculated on decode.
				PayerKey:  pubkey,
				Signature: &sig,
			},
		},
	}

	for _, testCase := range tests {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			// Calculate the merkle root for the invoice request
			// we're testing (rather than needing to pre-calculate
			// it for each test case), so that we can use
			// require.Equal to compare it to the decoded invoice
			// (which has its merkle root calculated on decode).
			records, err := testCase.encoded.records()
			require.NoError(t, err, "records")

			testCase.encoded.MerkleRoot, err = MerkleRoot(records)
			require.NoError(t, err, "merkle root")

			encodedBytes, err := EncodeInvoiceRequest(
				testCase.encoded,
			)
			require.NoError(t, err, "encode")

			decoded, err := DecodeInvoiceRequest(encodedBytes)
			require.NoError(t, err, "decode")

			// Our decoding creates an empty feature vector if no
			// features TLV is present so that we can use the
			// non-nil vector. If our test didn't set any features,
			// fill in an empty feature vector so that we can use
			// require.Equal for the encoded/decoded values.
			if testCase.encoded.Features == nil {
				testCase.encoded.Features =
					lnwire.NewFeatureVector(
						lnwire.NewRawFeatureVector(),
						lnwire.Features,
					)
			}

			require.Equal(t, testCase.encoded, decoded)
		})
	}
}

// TestInvoiceRequestValidation tests validation of invoice requests.
func TestInvoiceRequestValidation(t *testing.T) {
	var (
		hash, merkleRoot lntypes.Hash

		privkey = testutils.GetPrivkeys(t, 1)[0]
		pubkey  = privkey.PubKey()
	)

	copy(hash[:], []byte{1, 2, 3})

	// Populate a random merkle root value to test signature validation.
	copy(merkleRoot[:], []byte{3, 2, 1})
	digest := signatureDigest(invoiceRequestTag, signatureTag, merkleRoot)

	sig, err := schnorr.Sign(privkey, digest[:])
	require.NoError(t, err, "sign root")

	// Serialized our signature and copy into [64]byte.
	sigBytes := sig.Serialize()
	var schnorrSig [64]byte
	copy(schnorrSig[:], sigBytes)

	tests := []struct {
		name       string
		invoiceReq *InvoiceRequest
		err        error
	}{
		{
			name:       "no offer ID",
			invoiceReq: &InvoiceRequest{},
			err:        ErrOfferIDRequired,
		},
		{
			name: "no payer key",
			invoiceReq: &InvoiceRequest{
				OfferID: hash,
			},
			err: ErrPayerKeyRequired,
		},
		{
			name: "no signature",
			invoiceReq: &InvoiceRequest{
				OfferID:  hash,
				PayerKey: pubkey,
			},
			err: ErrSignatureRequired,
		},
		{
			name: "invalid signature",
			invoiceReq: &InvoiceRequest{
				OfferID:  hash,
				PayerKey: pubkey,
				// Use a valid signature over merkleRoot, but
				// then set our merkle root value to something
				// else to force an error.
				Signature:  &schnorrSig,
				MerkleRoot: hash,
			},
			err: ErrInvalidSig,
		},
		{
			name: "valid signature",
			invoiceReq: &InvoiceRequest{
				OfferID:    hash,
				PayerKey:   pubkey,
				Signature:  &schnorrSig,
				MerkleRoot: merkleRoot,
			},
		},
	}

	for _, testCase := range tests {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.invoiceReq.Validate()
			require.True(t, errors.Is(err, testCase.err))
		})
	}

}

// TestNewInvoiceRequest tests creation of an invoice request for a specific
// offer.
func TestNewInvoiceRequest(t *testing.T) {
	var (
		pubkeys = testutils.GetPubkeys(t, 2)
		offer   = &Offer{
			MinimumAmount: lnwire.MilliSatoshi(100),
			Description:   "offer description",
			Issuer:        "offer issuer",
			QuantityMin:   2,
			QuantityMax:   4,
			NodeID:        pubkeys[0],
		}

		validRequest = &InvoiceRequest{
			Amount:    offer.MinimumAmount + 5,
			Quantity:  offer.QuantityMin + 1,
			PayerKey:  pubkeys[1],
			PayerNote: "note",
		}

		offerNoQuantity = &Offer{
			MinimumAmount: lnwire.MilliSatoshi(100),
			Description:   "offer description",
			Issuer:        "offer issuer",
			QuantityMin:   0,
			QuantityMax:   0,
			NodeID:        pubkeys[0],
		}
	)

	// Calculate offer ID for our offer.
	records, err := offer.records()
	require.NoError(t, err, "offer records")

	offerID, err := MerkleRoot(records)
	require.NoError(t, err, "offer root")

	offer.MerkleRoot = offerID
	validRequest.OfferID = offerID

	// Calculate merkle root for our valid request.
	records, err = validRequest.records()
	require.NoError(t, err, "request records")

	validRequest.MerkleRoot, err = MerkleRoot(records)
	require.NoError(t, err, "request root")

	tests := []struct {
		name      string
		offer     *Offer
		amount    lnwire.MilliSatoshi
		quantity  uint64
		payerKey  *btcec.PublicKey
		payerNote string
		err       error
		expected  *InvoiceRequest
	}{
		{
			name:   "amount too small",
			offer:  offer,
			amount: offer.MinimumAmount - 10,
			err:    ErrBelowMinAmount,
		},
		{
			name:     "below min quantity",
			offer:    offer,
			amount:   offer.MinimumAmount,
			quantity: offer.QuantityMin - 1,
			err:      ErrOutsideQuantityRange,
		},
		{
			name:     "above max quantity",
			offer:    offer,
			amount:   offer.MinimumAmount,
			quantity: offer.QuantityMax + 1,
			err:      ErrOutsideQuantityRange,
		},
		{
			name:     "no quantity when required",
			offer:    offer,
			amount:   offer.MinimumAmount,
			quantity: 0,
			err:      ErrQuantityRequired,
		},
		{
			name:     "quantity when not required",
			offer:    offerNoQuantity,
			amount:   offerNoQuantity.MinimumAmount,
			quantity: 1,
			err:      ErrNoQuantity,
		},
		{
			name:      "request ok",
			offer:     offer,
			amount:    offer.MinimumAmount + 5,
			quantity:  offer.QuantityMin + 1,
			payerKey:  pubkeys[1],
			payerNote: "note",
			err:       nil,
			expected:  validRequest,
		},
	}

	for _, testCase := range tests {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			actual, err := NewInvoiceRequest(
				testCase.offer, testCase.amount,
				testCase.quantity, testCase.payerKey,
				testCase.payerNote,
			)
			require.True(t, errors.Is(err, testCase.err))
			require.Equal(t, testCase.expected, actual)
		})
	}
}

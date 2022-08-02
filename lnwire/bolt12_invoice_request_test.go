package lnwire

import (
	"testing"

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

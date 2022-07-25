package offers

import (
	"testing"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/carlakc/boltnd/testutils"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/stretchr/testify/require"
)

// TestInvoiceRequestEncoding tests encoding and decoding of invoice requests.
// Test cases use individual fields to implicitly test the case where other
// fields are not included.
func TestInvoiceRequestEncoding(t *testing.T) {
	var (
		offerIDBytes = [32]byte{1, 2, 3}
		signature    = [64]byte{3, 2, 1}
	)

	offerID, err := chainhash.NewHash(offerIDBytes[:])
	require.NoError(t, err, "offer ID")

	tests := []struct {
		name    string
		request *InvoiceRequest
	}{
		{
			name: "offer ID",
			request: &InvoiceRequest{
				OfferID: offerID,
			},
		},
		{
			name: "amount",
			request: &InvoiceRequest{
				Amount: lnwire.MilliSatoshi(3),
			},
		},
		{
			name: "features",
			request: &InvoiceRequest{
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
			request: &InvoiceRequest{
				Quantity: 10,
			},
		},
		{
			name: "payer key",
			request: &InvoiceRequest{
				PayerKey: testutils.GetPubkeys(t, 1)[0],
			},
		},
		{
			name: "payer note",
			request: &InvoiceRequest{
				PayerNote: "note",
			},
		},
		{
			name: "payer info",
			request: &InvoiceRequest{
				PayerInfo: []byte{1, 2, 3},
			},
		},
		{
			name: "signature",
			request: &InvoiceRequest{
				// Include a non-signature field so that we can
				// calculate a merkle root when we decode.
				PayerKey:  testutils.GetPubkeys(t, 1)[0],
				Signature: &signature,
			},
		},
	}

	for _, testCase := range tests {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			encoded, err := EncodeInvoiceRequest(testCase.request)
			require.NoError(t, err, "encode")

			decoded, err := DecodeInvoiceRequest(encoded)
			require.NoError(t, err, "decode")

			// When we decode an invoice request, we always set
			// non-nil feature vector, so we update our test's
			// input so that we can use require.Equal.
			if testCase.request.Features == nil {
				emptyFeatures := lnwire.NewFeatureVector(
					lnwire.NewRawFeatureVector(),
					lnwire.Features,
				)

				testCase.request.Features = emptyFeatures
			}

			// Clear the decoded merkle root (because we're not
			// testing root calculation here).
			decoded.MerkleRoot = chainhash.Hash{}

			require.Equal(t, testCase.request, decoded)
		})
	}
}

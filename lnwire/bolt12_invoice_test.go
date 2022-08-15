package lnwire

import (
	"errors"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/carlakc/boltnd/testutils"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/stretchr/testify/require"
)

// TestInvoiceEncoding tests encoding and decoding of bolt 12 invoices. Test
// cases generally contain a single field per case to implicitly test the case
// where a field is not included.
func TestInvoiceEncoding(t *testing.T) {
	var (
		pubkey = testutils.GetPubkeys(t, 1)[0]

		hash lntypes.Hash

		sig [64]byte
	)

	copy(hash[:], []byte{1, 2, 3})
	copy(sig[:], []byte{4, 5, 6})

	tests := []struct {
		name    string
		encoded *Invoice
	}{
		{
			name: "chain hash",
			encoded: &Invoice{
				Chainhash: hash,
			},
		},
		{
			name: "offer id",
			encoded: &Invoice{
				OfferID: hash,
			},
		},
		{
			name: "amount",
			encoded: &Invoice{
				Amount: lnwire.MilliSatoshi(1),
			},
		},
		{
			name: "description",
			encoded: &Invoice{
				Description: "inv description",
			},
		},
		{
			name: "features - empty",
			encoded: &Invoice{
				// Include a non-empty field so that our merkle
				// tree can be calculated.
				Description: "inv description",
				Features: lnwire.NewFeatureVector(
					lnwire.NewRawFeatureVector(),
					lnwire.Features,
				),
			},
		},
		{
			name: "features - populated",
			encoded: &Invoice{
				Features: lnwire.NewFeatureVector(
					lnwire.NewRawFeatureVector(
						lnwire.AMPOptional,
					),
					lnwire.Features,
				),
			},
		},
		{
			name: "node id",
			encoded: &Invoice{
				NodeID: pubkey,
			},
		},
		{
			name: "quantity",
			encoded: &Invoice{
				Quantity: 3,
			},
		},
		{
			name: "payer key",
			encoded: &Invoice{
				PayerKey: pubkey,
			},
		},
		{
			name: "payer note",
			encoded: &Invoice{
				PayerNote: "note",
			},
		},
		{
			name: "created at",
			encoded: &Invoice{
				CreatedAt: time.Date(
					2022, 01, 01, 0, 0, 0, 0, time.Local,
				),
			},
		},
		{
			name: "payment hash",
			encoded: &Invoice{
				PaymentHash: hash,
			},
		},
		{
			name: "relative expiry",
			encoded: &Invoice{
				RelativeExpiry: time.Second * 20,
			},
		},
		{
			name: "cltv expiry",
			encoded: &Invoice{
				CLTVExpiry: 10,
			},
		},
		{
			name: "payer info",
			encoded: &Invoice{
				PayerInfo: []byte{1, 2, 3},
			},
		},
		{
			name: "signature",
			encoded: &Invoice{
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
			// Calculate the merkle root for the invoice we're
			// testing (rather than needing to pre-calculate it
			// for each test case), so that we can use require.Equal
			// to compare it to the decoded invoice (which has its
			// merkle root calculated on decode).
			records, err := testCase.encoded.records()
			require.NoError(t, err, "records")

			testCase.encoded.MerkleRoot, err = MerkleRoot(records)
			require.NoError(t, err, "merkle root")

			encodedBytes, err := EncodeInvoice(testCase.encoded)
			require.NoError(t, err, "encode")

			decoded, err := DecodeInvoice(encodedBytes)
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

// TestInvoiceValidation tests validation of bolt 12 invoices.
func TestInvoiceValidation(t *testing.T) {
	var (
		created = time.Date(
			2022, 01, 01, 0, 0, 0, 0, time.Local,
		)

		hash, merkleRoot lntypes.Hash

		privkey = testutils.GetPrivkeys(t, 1)[0]
		pubkey  = privkey.PubKey()
	)

	copy(hash[:], []byte{1, 2, 3})

	// Populate a random merkle root value to test signature validation.
	copy(merkleRoot[:], []byte{3, 2, 1})
	digest := signatureDigest(invoiceTag, signatureTag, merkleRoot)

	sig, err := schnorr.Sign(privkey, digest[:])
	require.NoError(t, err, "sign root")

	// Serialized our signature and copy into [64]byte.
	sigBytes := sig.Serialize()
	var schnorrSig [64]byte
	copy(schnorrSig[:], sigBytes)

	tests := []struct {
		name    string
		invoice *Invoice
		err     error
	}{
		{
			name:    "no amount",
			invoice: &Invoice{},
			err:     ErrNoAmount,
		},
		{
			name: "no payment hash",
			invoice: &Invoice{
				Amount: lnwire.MilliSatoshi(1),
			},
			err: ErrNoPaymentHash,
		},
		{
			name: "no creation time",
			invoice: &Invoice{
				Amount:      lnwire.MilliSatoshi(1),
				PaymentHash: hash,
			},
			err: ErrNoCreationTime,
		},
		{
			name: "no node id",
			invoice: &Invoice{
				Amount:      lnwire.MilliSatoshi(1),
				PaymentHash: hash,
				CreatedAt:   created,
			},
			err: ErrNodeIDRequired,
		},
		{
			name: "no description",
			invoice: &Invoice{
				Amount:      lnwire.MilliSatoshi(1),
				PaymentHash: hash,
				CreatedAt:   created,
				NodeID:      pubkey,
			},
			err: ErrDescriptionRequried,
		},
		{
			name: "invalid signature",
			invoice: &Invoice{
				Amount:      lnwire.MilliSatoshi(1),
				PaymentHash: hash,
				CreatedAt:   created,
				NodeID:      pubkey,
				Description: "invoice",
				// Use a signature that has signed the digest
				// of our merkleRoot var.
				Signature: &schnorrSig,
				// Set our merkle root to a different value to
				// merkleRoot (above) to test for invalid
				// signatures.
				MerkleRoot: hash,
			},
			err: ErrInvalidSig,
		},
		{
			name: "signature valid",
			invoice: &Invoice{
				Amount:      lnwire.MilliSatoshi(1),
				PaymentHash: hash,
				CreatedAt:   created,
				NodeID:      pubkey,
				Description: "invoice",
				// Use a signature that has signed the digest
				// of our merkleRoot var, and set the correct
				// merkle root to produce a valid signature.
				Signature:  &schnorrSig,
				MerkleRoot: merkleRoot,
			},
		},
	}

	for _, testCase := range tests {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.invoice.Validate()
			require.True(t, errors.Is(err, testCase.err))
		})
	}
}

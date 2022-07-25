package offers

import (
	"math"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/carlakc/boltnd/testutils"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/stretchr/testify/require"
)

// TestOfferEncoding tests encoding and decoding of offers. It tests each field
// in the offer individually so that each test also implicitly tests the case
// where other fields are not set.
func TestOfferEncoding(t *testing.T) {
	sig := [64]byte{4, 5, 6}

	// Pubkeys are expressed as x-only.
	pubkey := testutils.GetPubkeys(t, 1)[0]
	nodeID, err := schnorr.ParsePubKey(schnorr.SerializePubKey(pubkey))
	require.NoError(t, err, "xonly pubkey")

	tests := []struct {
		name  string
		offer *Offer
	}{
		{
			name: "min amount - zeros truncated",
			offer: &Offer{
				MinimumAmount: lnwire.MilliSatoshi(1),
			},
		},
		{
			name: "min amount - zeros not truncates",
			offer: &Offer{
				MinimumAmount: lnwire.MilliSatoshi(
					math.MaxInt64,
				),
			},
		},
		{
			name: "description",
			offer: &Offer{
				Description: "offer description",
			},
		},
		{
			name: "features vector",
			offer: &Offer{
				Features: lnwire.NewFeatureVector(
					// Set any random feature bit to test
					// encoding.
					lnwire.NewRawFeatureVector(
						lnwire.TLVOnionPayloadRequired,
					),
					lnwire.Features,
				),
			},
		},
		{
			name: "expiry",
			offer: &Offer{
				Expiry: time.Unix(900, 0),
			},
		},
		{
			name: "issuer",
			offer: &Offer{
				Issuer: "issuer",
			},
		},
		{
			name: "quantity - min / max",
			offer: &Offer{
				QuantityMin: 1,
				QuantityMax: 3,
			},
		},
		{
			name: "node ID",
			offer: &Offer{
				NodeID: nodeID,
			},
		},
		{
			name: "node sig",
			offer: &Offer{
				// We include another field here because valid
				// offers require a non-sig TLV to calculate
				// merkle root.
				NodeID:    nodeID,
				Signature: &sig,
			},
		},
	}

	for _, testCase := range tests {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			encoded, err := EncodeOffer(testCase.offer)
			require.NoError(t, err, "encode")

			decoded, err := DecodeOffer(encoded)
			require.NoError(t, err, "decode")

			// Our decoding creates an empty feature vector if no
			// features TLV is present so that we can use the
			// non-nil vector. If our test didn't set any features,
			// fill in an empty feature vector so that we can use
			// require.Equal for the encoded/decoded values.
			if testCase.offer.Features == nil {
				testCase.offer.Features = lnwire.NewFeatureVector(
					lnwire.NewRawFeatureVector(),
					lnwire.Features,
				)
			}

			// We also clear our merkle root value which is
			// calculated when we decode the offer tlv stream, we're
			// not testing this calculation here.
			decoded.MerkleRoot = chainhash.Hash{}

			require.Equal(t, testCase.offer, decoded)
		})
	}
}

// TestDecodedMerkleRoot tests that the tlv merkle root is the same for an
// offer once it has been encoded/decoded.
func TestDecodedMerkleRoot(t *testing.T) {
	// Create an arbitrary offer and calculate its merkle root.
	offer := &Offer{
		Description:   "description string",
		MinimumAmount: lnwire.MilliSatoshi(10),
	}

	records, err := offer.records()
	require.NoError(t, err, "get records")

	merkleRoot, err := MerkleRoot(records)
	require.NoError(t, err)

	// Now encode and decode the offer to check that we get the same root
	// after decoding.
	offerBytes, err := EncodeOffer(offer)
	require.NoError(t, err, "encode")

	decodedOffer, err := DecodeOffer(offerBytes)
	require.NoError(t, err, "decode")

	require.Equal(t, *merkleRoot, decodedOffer.MerkleRoot)
}

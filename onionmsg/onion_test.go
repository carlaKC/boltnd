package onionmsg

import (
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/carlakc/boltnd/testutils"
	sphinx "github.com/lightningnetwork/lightning-onion"
	"github.com/stretchr/testify/require"
)

// mockedPayloadEncode is a mocked encode function for blinded hop paylaods
// which just returns the compressed serialization of the public key provided.
func mockedPayloadEncode(pk *btcec.PublicKey) ([]byte, error) {
	return pk.SerializeCompressed(), nil
}

// TestCreatePathToBlind tests formation of blinded route paths from a set of
// pubkeys.
func TestCreatePathToBlind(t *testing.T) {
	pubkeys := testutils.GetPubkeys(t, 3)

	tests := []struct {
		name         string
		route        []*btcec.PublicKey
		expectedPath []*sphinx.BlindedPathHop
	}{
		{
			// A single hop blinded path will just have the first
			// node's pubkey, there is no extra data because they
			// are the terminal hop.
			name: "one hop",
			route: []*btcec.PublicKey{
				pubkeys[0],
			},
			expectedPath: []*sphinx.BlindedPathHop{
				{
					NodePub: pubkeys[0],
				},
			},
		},
		{
			name: "three hops",
			route: []*btcec.PublicKey{
				pubkeys[0],
				pubkeys[1],
				pubkeys[2],
			},
			expectedPath: []*sphinx.BlindedPathHop{
				{
					NodePub: pubkeys[0],
					Payload: pubkeys[1].SerializeCompressed(),
				},
				{
					NodePub: pubkeys[1],
					Payload: pubkeys[2].SerializeCompressed(),
				},
				{
					NodePub: pubkeys[2],
				},
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			actualPath, err := createPathToBlind(
				testCase.route, mockedPayloadEncode,
			)
			require.NoError(t, err, "create path")

			require.Equal(t, testCase.expectedPath, actualPath)
		})
	}
}

// TestBlindedToSphinx tests conversion of a blinded path to a sphinx path.
func TestBlindedToSphinx(t *testing.T) {
	pubkeys := testutils.GetPubkeys(t, 4)

	var (
		payload0 = []byte{0, 0, 0}
		payload1 = []byte{1, 1, 1}
		payload2 = []byte{2, 2, 2}
	)

	tests := []struct {
		name         string
		blindedPath  *sphinx.BlindedPath
		expectedPath *sphinx.PaymentPath
	}{
		{
			// We should use the blinded pubkey for our introduction
			// node for onion messages.
			name: "only introduction point",
			blindedPath: &sphinx.BlindedPath{
				IntroductionPoint: pubkeys[0],
				EncryptedData: [][]byte{
					payload0,
				},
				BlindedHops: []*btcec.PublicKey{
					pubkeys[1],
				},
			},
			expectedPath: &sphinx.PaymentPath{
				{
					NodePub: *pubkeys[1],
					HopPayload: sphinx.HopPayload{
						Type:    sphinx.PayloadTLV,
						Payload: payload0,
					},
				},
			},
		},
		{
			name: "three hops",
			blindedPath: &sphinx.BlindedPath{
				IntroductionPoint: pubkeys[0],
				EncryptedData: [][]byte{
					payload0, payload1, payload2,
				},
				BlindedHops: []*btcec.PublicKey{
					pubkeys[1],
					pubkeys[2],
					pubkeys[3],
				},
			},
			expectedPath: &sphinx.PaymentPath{
				{
					NodePub: *pubkeys[1],
					HopPayload: sphinx.HopPayload{
						Type:    sphinx.PayloadTLV,
						Payload: payload0,
					},
				},
				{
					NodePub: *pubkeys[2],
					HopPayload: sphinx.HopPayload{
						Type:    sphinx.PayloadTLV,
						Payload: payload1,
					},
				},
				{
					NodePub: *pubkeys[3],
					HopPayload: sphinx.HopPayload{
						Type:    sphinx.PayloadTLV,
						Payload: payload2,
					},
				},
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			actualPath, err := blindedToSphinx(testCase.blindedPath)
			require.NoError(t, err)

			require.Equal(t, testCase.expectedPath, actualPath)
		})
	}
}

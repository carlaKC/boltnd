package onionmsg

import (
	"errors"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/carlakc/boltnd/lnwire"
	"github.com/carlakc/boltnd/testutils"
	sphinx "github.com/lightningnetwork/lightning-onion"
	"github.com/lightningnetwork/lnd/tlv"
	"github.com/stretchr/testify/require"
)

// mockedPayloadEncode is a mocked encode function for blinded hop paylaods
// which just returns the compressed serialization of the public key provided.
func mockedPayloadEncode(data *lnwire.BlindedRouteData) ([]byte, error) {
	return data.NextNodeID.SerializeCompressed(), nil
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
		// Create encrypted payloads for each hop.
		encryptedData0 = []byte{0, 0, 0}
		encryptedData1 = []byte{1, 1, 1}
		encryptedData2 = []byte{2, 2, 2}

		// Create onion payloads containing our encrypted data for each
		// hop.
		onionPayload0 = &lnwire.OnionMessagePayload{
			EncryptedData: encryptedData0,
		}

		onionPayload1 = &lnwire.OnionMessagePayload{
			EncryptedData: encryptedData1,
		}

		onionPayload2 = &lnwire.OnionMessagePayload{
			EncryptedData: encryptedData2,
		}

		// Add an arbitrary final payload intended for the last hop.
		finalPayload = []*lnwire.FinalHopPayload{
			{
				TLVType: tlv.Type(101),
				Value:   []byte{9, 9, 9},
			},
		}

		// Create two payloads which utilize the final payload with
		// different sets of encrypted data.
		onion0WithFinal = &lnwire.OnionMessagePayload{
			EncryptedData:    encryptedData0,
			FinalHopPayloads: finalPayload,
		}

		onion1WithFinal = &lnwire.OnionMessagePayload{
			EncryptedData:    encryptedData1,
			FinalHopPayloads: finalPayload,
		}

		// Create a reply path an onion payload including it.
		replyPath = &lnwire.ReplyPath{
			FirstNodeID:   pubkeys[0],
			BlindingPoint: pubkeys[1],
		}

		onion1WithReplyPath = &lnwire.OnionMessagePayload{
			EncryptedData: encryptedData1,
			ReplyPath:     replyPath,
		}
	)

	// Finally, encode all payloads for our test as tlv streams.
	payload0, err := lnwire.EncodeOnionMessagePayload(onionPayload0)
	require.NoError(t, err, "payload 0")

	payload1, err := lnwire.EncodeOnionMessagePayload(onionPayload1)
	require.NoError(t, err, "payload 1")

	payload2, err := lnwire.EncodeOnionMessagePayload(onionPayload2)
	require.NoError(t, err, "payload 2")

	payload0WithFinal, err := lnwire.EncodeOnionMessagePayload(
		onion0WithFinal,
	)
	require.NoError(t, err, "payload 0 with final payload")

	payload1WithFinal, err := lnwire.EncodeOnionMessagePayload(
		onion1WithFinal,
	)
	require.NoError(t, err, "payload 1 with final payload")

	payload1WithReplyPath, err := lnwire.EncodeOnionMessagePayload(
		onion1WithReplyPath,
	)
	require.NoError(t, err, "payload 1 with reply path")

	tests := []struct {
		name         string
		blindedPath  *sphinx.BlindedPath
		replyPath    *lnwire.ReplyPath
		finalPayload []*lnwire.FinalHopPayload
		expectedPath *sphinx.PaymentPath
	}{
		{
			// We should use the blinded pubkey for our introduction
			// node for onion messages.
			name: "only introduction point",
			blindedPath: &sphinx.BlindedPath{
				IntroductionPoint: pubkeys[0],
				EncryptedData: [][]byte{
					encryptedData0,
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
			name: "single hop with final payload",
			blindedPath: &sphinx.BlindedPath{
				IntroductionPoint: pubkeys[0],
				EncryptedData: [][]byte{
					encryptedData0,
				},
				BlindedHops: []*btcec.PublicKey{
					pubkeys[1],
				},
			},
			finalPayload: finalPayload,
			expectedPath: &sphinx.PaymentPath{
				{
					NodePub: *pubkeys[1],
					HopPayload: sphinx.HopPayload{
						Type:    sphinx.PayloadTLV,
						Payload: payload0WithFinal,
					},
				},
			},
		},
		{
			name: "two hops with final payload",
			blindedPath: &sphinx.BlindedPath{
				IntroductionPoint: pubkeys[0],
				EncryptedData: [][]byte{
					encryptedData0, encryptedData1,
				},
				BlindedHops: []*btcec.PublicKey{
					pubkeys[1],
					pubkeys[2],
				},
			},
			finalPayload: finalPayload,
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
						Payload: payload1WithFinal,
					},
				},
			},
		},
		{
			name: "three hops",
			blindedPath: &sphinx.BlindedPath{
				IntroductionPoint: pubkeys[0],
				EncryptedData: [][]byte{
					encryptedData0, encryptedData1,
					encryptedData2,
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
		{
			name: "reply path",
			blindedPath: &sphinx.BlindedPath{
				IntroductionPoint: pubkeys[0],
				EncryptedData: [][]byte{
					encryptedData0, encryptedData1,
				},
				BlindedHops: []*btcec.PublicKey{
					pubkeys[1], pubkeys[2],
				},
			},
			replyPath: replyPath,
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
						Payload: payload1WithReplyPath,
					},
				},
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			actualPath, err := blindedToSphinx(
				testCase.blindedPath, testCase.replyPath,
				testCase.finalPayload,
			)
			require.NoError(t, err)

			require.Equal(t, testCase.expectedPath, actualPath)
		})
	}
}

// TestDecryptBlob tests decrypting of onion message blobs.
func TestDecryptBlob(t *testing.T) {
	var (
		privkeys = testutils.GetPrivkeys(t, 3)
		nodeECDH = &sphinx.PrivKeyECDH{
			PrivKey: privkeys[0],
		}

		blindingPrivkey = privkeys[1]
	)

	tests := []struct {
		name    string
		payload *lnwire.OnionMessagePayload
		err     error
	}{
		{
			name:    "no payload",
			payload: nil,
			err:     ErrNoForwardingPayload,
		},
		{
			name: "no encrypted data",
			payload: &lnwire.OnionMessagePayload{
				EncryptedData: nil,
			},
			err: ErrNoEncryptedData,
		},
	}

	for _, testCase := range tests {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			decryptBlob := decryptBlobFunc(nodeECDH)

			_, err := decryptBlob(
				blindingPrivkey.PubKey(), testCase.payload,
			)
			require.True(t, errors.Is(err, testCase.err))
		})
	}
}

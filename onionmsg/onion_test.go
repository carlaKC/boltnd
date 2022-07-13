package onionmsg

import (
	"github.com/stretchr/testify/assert"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/carlakc/boltnd/testutils"
	sphinx "github.com/lightningnetwork/lightning-onion"
	"github.com/lightningnetwork/lnd/routing/route"
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
			name: "only introduction point",
			blindedPath: &sphinx.BlindedPath{
				IntroductionPoint: pubkeys[0],
				EncryptedData: [][]byte{
					payload0,
				},
			},
			expectedPath: &sphinx.PaymentPath{
				{
					NodePub: *pubkeys[0],
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
					// Note that the first blinded hop is
					// the introduction node, which should
					// not be used in our sphinx path. We
					// add an extra pubkey here so that
					// it'll be detected if used.
					pubkeys[3],
					pubkeys[1],
					pubkeys[2],
				},
			},
			expectedPath: &sphinx.PaymentPath{
				{
					NodePub: *pubkeys[0],
					HopPayload: sphinx.HopPayload{
						Type:    sphinx.PayloadTLV,
						Payload: payload0,
					},
				},
				{
					NodePub: *pubkeys[1],
					HopPayload: sphinx.HopPayload{
						Type:    sphinx.PayloadTLV,
						Payload: payload1,
					},
				},
				{
					NodePub: *pubkeys[2],
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

// TestHmacStuff is a unit test used to test unwrapping onions and figure out
// why we're getting invalid hmac errors.
func TestHmacStuff(t *testing.T) {
	// Create a random private key for the receiver.
	receiver, err := btcec.NewPrivateKey()
	require.NoError(t, err, "private key")

	receiverPubkey, err := route.NewVertexFromBytes(
		receiver.PubKey().SerializeCompressed(),
	)
	require.NoError(t, err, "public key")

	// Create a custom onion message for our peer.
	onionMsg, err := customOnionMessage(receiverPubkey)
	require.NoError(t, err, "create onion")

	// Create ECDH interface ops for receiver.
	receiverNodeKey := &sphinx.PrivKeyECDH{
		PrivKey: receiver,
	}

	// Mock out lnd and create a messenger for our receiver.
	lnd := testutils.NewMockLnd()
	messenger := NewOnionMessenger(
		&chaincfg.RegressionNetParams, lnd, receiverNodeKey, func(err error) {
			t.Log(err)
		},
	)

	testutils.MockSubscribeCustomMessages(lnd.Mock, nil, nil, nil)

	require.NoError(t, messenger.Start(), "start msngr")
	defer func() {
		assert.NoError(t, messenger.Stop(), "stop msngr")
	}()

	err = messenger.handleOnionMessage(*onionMsg)
	require.NoError(t, err, "handle msg")
}

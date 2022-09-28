package routes

import (
	"context"
	"errors"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/carlakc/boltnd/lnwire"
	"github.com/carlakc/boltnd/testutils"
	"github.com/lightninglabs/lndclient"
	sphinx "github.com/lightningnetwork/lightning-onion"
	lndwire "github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/routing/route"
	"github.com/lightningnetwork/lnd/tlv"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestGetRelayingPeers tests filtering of peers into a set that can relay
// onion messages to us.
func TestGetRelayingPeers(t *testing.T) {
	var (
		pubkeys  = testutils.GetPubkeys(t, 2)
		channel1 = lndclient.ChannelInfo{
			ChannelID:   1,
			PubKeyBytes: route.NewVertex(pubkeys[0]),
		}

		channel1NodeInfo = &lndclient.NodeInfo{
			Node: &lndclient.Node{
				PubKey: channel1.PubKeyBytes,
			},
		}

		channel2 = lndclient.ChannelInfo{
			ChannelID:   2,
			PubKeyBytes: route.NewVertex(pubkeys[1]),
		}

		channel2NodeInfo = &lndclient.NodeInfo{
			Node: &lndclient.Node{
				PubKey: channel2.PubKeyBytes,
			},
		}

		mockErr = errors.New("err")
	)

	tests := []struct {
		name      string
		setupMock func(m *mock.Mock)
		canRelay  func(*lndclient.NodeInfo) error
		peers     []*lndclient.NodeInfo
		err       error
	}{
		{
			name: "no active channels",
			setupMock: func(m *mock.Mock) {
				testutils.MockListChannels(
					m, true, false,
					[]lndclient.ChannelInfo{}, nil,
				)
			},
			err: ErrNoChannels,
		},
		{
			name: "active channels - can't relay",
			setupMock: func(m *mock.Mock) {
				testutils.MockListChannels(
					m, true, false,
					[]lndclient.ChannelInfo{
						channel1,
					}, nil,
				)

				// Expect our channel's node to be looked up.
				testutils.MockGetNodeInfo(
					m, channel1.PubKeyBytes, true,
					channel1NodeInfo, nil,
				)
			},
			// Return false for all channels.
			canRelay: func(*lndclient.NodeInfo) error {
				return errors.New("can't relay")
			},
			peers: nil,
			err:   nil,
		},
		{
			name: "active channels - one can relay",
			setupMock: func(m *mock.Mock) {
				// Return two channels.
				testutils.MockListChannels(
					m, true, false,
					[]lndclient.ChannelInfo{
						channel1, channel2,
					}, nil,
				)

				// Prime our mock to find both channels peers.
				testutils.MockGetNodeInfo(
					m, channel1.PubKeyBytes, true,
					channel1NodeInfo, nil,
				)

				testutils.MockGetNodeInfo(
					m, channel2.PubKeyBytes, true,
					channel2NodeInfo, nil,
				)
			},
			// Set our relay filter to only allow channel 1.
			canRelay: func(info *lndclient.NodeInfo) error {
				if info.PubKey == channel1.PubKeyBytes {
					return nil
				}

				return errors.New("can't relay")
			},
			peers: []*lndclient.NodeInfo{
				channel1NodeInfo,
			},
			err: nil,
		},
		{
			name: "channel's node not found",
			setupMock: func(m *mock.Mock) {
				// Return two channels.
				testutils.MockListChannels(
					m, true, false,
					[]lndclient.ChannelInfo{
						channel1, channel2,
					}, nil,
				)

				// Prime our mock to lookup the first one
				// successfully.
				testutils.MockGetNodeInfo(
					m, channel1.PubKeyBytes, true,
					channel1NodeInfo, nil,
				)

				// Prime our mock to return a not found error
				// for our second channel.
				err := status.Error(codes.NotFound, "err str")
				testutils.MockGetNodeInfo(
					m, channel2.PubKeyBytes, true,
					// Our channel array is non-nil so
					// that the mock can cast this value
					// without nil-checks.
					&lndclient.NodeInfo{}, err,
				)
			},
			// Set all peers able to relay.
			canRelay: func(*lndclient.NodeInfo) error {
				return nil
			},
			// We should succeed with one of our channels despite
			// not being able to find the other one.
			peers: []*lndclient.NodeInfo{
				channel1NodeInfo,
			},
			err: nil,
		},
		{
			// Test the case where we get a failure other than
			// "not found" from GetNodeInfo and ensure that we
			// error out accordingly.
			name: "node info failure",
			setupMock: func(m *mock.Mock) {
				// Return two channels.
				testutils.MockListChannels(
					m, true, false,
					[]lndclient.ChannelInfo{
						channel1, channel2,
					}, nil,
				)

				// Prime our mock to lookup the first one
				// successfully.
				testutils.MockGetNodeInfo(
					m, channel1.PubKeyBytes, true,
					&lndclient.NodeInfo{}, mockErr,
				)
			},
			err: mockErr,
		},
	}

	for _, testCase := range tests {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			lnd := testutils.NewMockLnd()
			defer lnd.AssertExpectations(t)

			testCase.setupMock(lnd.Mock)

			ctx := context.Background()
			peers, err := getRelayingPeers(
				ctx, lnd, testCase.canRelay,
			)

			require.True(t, errors.Is(err, testCase.err))
			require.Equal(t, testCase.peers, peers)
		})
	}
}

// TestCreateRelayCheck tests the canRelay closure used to filter peers.
func TestCreateRelayCheck(t *testing.T) {
	var (
		// nodeChannels is a non-empty set of channels, we don't fill
		// values because they're not used.
		nodeChannels = []lndclient.ChannelEdge{
			{}, {},
		}
	)
	tests := []struct {
		name     string
		features []lndwire.FeatureBit
		nodeInfo *lndclient.NodeInfo
		err      error
	}{
		{
			name: "no channels",
			nodeInfo: &lndclient.NodeInfo{
				Channels: nil,
			},
			err: ErrNoPeerChannels,
		},
		{
			name: "no node info",
			nodeInfo: &lndclient.NodeInfo{
				Channels: nodeChannels,
				Node:     nil,
			},
			err: ErrNoNodeInfo,
		},
		{
			features: []lndwire.FeatureBit{
				lndwire.AnchorsRequired,
				lndwire.DataLossProtectRequired,
			},
			nodeInfo: &lndclient.NodeInfo{
				Channels: nodeChannels,
				Node: &lndclient.Node{
					Features: []lndwire.FeatureBit{
						lndwire.AnchorsRequired,
					},
				},
			},
			err: ErrFeatureMismatch,
		},
		{
			// Test the case where the node has all our required
			// features, but might not have some of our optional
			// ones.
			name: "features ok",
			features: []lndwire.FeatureBit{
				lndwire.AnchorsRequired,
				lndwire.AMPOptional,
			},
			nodeInfo: &lndclient.NodeInfo{
				Channels: nodeChannels,
				Node: &lndclient.Node{
					Features: []lndwire.FeatureBit{
						lndwire.AnchorsRequired,
					},
				},
			},
			err: nil,
		},
	}

	for _, testCase := range tests {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			canRelay := createRelayCheck(testCase.features)
			err := canRelay(testCase.nodeInfo)
			require.True(t, errors.Is(err, testCase.err))
		})
	}
}

// TestBuildBlindedRoute tests construction of a blinded route to our node.
func TestBuildBlindedRoute(t *testing.T) {
	pubkeys := testutils.GetPubkeys(t, 3)
	introPayload := &lnwire.BlindedRouteData{
		NextNodeID: pubkeys[0],
	}

	introData, err := lnwire.EncodeBlindedRouteData(introPayload)
	require.NoError(t, err)

	tests := []struct {
		name          string
		relayingPeers []*lndclient.NodeInfo
		route         []*sphinx.BlindedPathHop
		err           error
	}{
		{
			name: "no relaying peers",
			err:  ErrNoRelayingPeers,
		},
		{
			name: "route with most channels peer",
			relayingPeers: []*lndclient.NodeInfo{
				{
					Channels: []lndclient.ChannelEdge{
						{}, {},
					},
					Node: &lndclient.Node{
						PubKey: route.NewVertex(
							pubkeys[1],
						),
					},
				},
				{
					Channels: []lndclient.ChannelEdge{
						{}, {}, {},
					},
					Node: &lndclient.Node{
						PubKey: route.NewVertex(
							pubkeys[2],
						),
					},
				},
			},
			route: []*sphinx.BlindedPathHop{
				{
					NodePub: pubkeys[2],
					Payload: introData,
				},
				{
					NodePub: pubkeys[0],
					Payload: nil,
				},
			},
		},
	}

	for _, testCase := range tests {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			route, err := buildBlindedRoute(
				testCase.relayingPeers, pubkeys[0],
			)

			require.True(t, errors.Is(err, testCase.err))
			require.Equal(t, route, testCase.route)
		})
	}
}

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

package routes

import (
	"context"
	"errors"
	"testing"

	"github.com/carlakc/boltnd/testutils"
	"github.com/lightninglabs/lndclient"
	lndwire "github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/routing/route"
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

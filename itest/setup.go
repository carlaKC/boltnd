package itest

import (
	"testing"

	"github.com/carlakc/boltnd/offersrpc"
	"github.com/lightningnetwork/lnd/lntest"
	"github.com/stretchr/testify/require"
)

const (
	onionMsgProtocolOverride = "--protocol.custom-message=513"
)

type bolt12TestSetup struct {
	aliceOffers offersrpc.OffersClient
	bobOffers   offersrpc.OffersClient
	cleanup     func()
}

// setupForBolt12 restarts our nodes with the appropriate overrides required to
// use bolt 12 functionality and returns connections to each node's offers
// subserver.
func setupForBolt12(t *testing.T, net *lntest.NetworkHarness) *bolt12TestSetup {
	// Update both nodes extra args to allow external handling of onion
	// messages and restart them so that the args some into effect.
	net.Alice.Cfg.ExtraArgs = []string{
		onionMsgProtocolOverride,
	}

	net.Bob.Cfg.ExtraArgs = []string{
		onionMsgProtocolOverride,
	}

	err := net.RestartNode(net.Alice, nil, nil)
	require.NoError(t, err, "alice restart")

	err = net.RestartNode(net.Bob, nil, nil)
	require.NoError(t, err, "bob restart")

	// Next, connect to each node's offers subserver.
	aliceConn, err := net.Alice.ConnectRPC(true)
	require.NoError(t, err, "alice grpc conn")

	bobConn, err := net.Bob.ConnectRPC(true)
	require.NoError(t, err, "bob grpc conn")

	return &bolt12TestSetup{
		aliceOffers: offersrpc.NewOffersClient(aliceConn),
		bobOffers:   offersrpc.NewOffersClient(bobConn),
		cleanup: func() {
			if err := aliceConn.Close(); err != nil {
				t.Logf("alice grpc conn close: %v", err)
			}

			if err := bobConn.Close(); err != nil {
				t.Logf("bob grpc conn close: %v", err)
			}
		},
	}
}

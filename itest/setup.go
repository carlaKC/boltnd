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
	aliceClient, aliceClean := bolt12Client(t, net.Alice)
	bobClient, bobClean := bolt12Client(t, net.Bob)

	return &bolt12TestSetup{
		aliceOffers: aliceClient,
		bobOffers:   bobClient,
		cleanup: func() {
			aliceClean()
			bobClean()
		},
	}
}

// bolt12Client returns an offersrpc client and cleanup for a node.
func bolt12Client(t *testing.T, node *lntest.HarnessNode) (
	offersrpc.OffersClient, func()) {

	conn, err := node.ConnectRPC(true)
	require.NoError(t, err, "%v grpc conn", node.Name())

	return offersrpc.NewOffersClient(conn), func() {
		if err := conn.Close(); err != nil {
			t.Logf("%v grpc conn close %v", node.Name(), err)
		}
	}
}

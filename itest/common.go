package itest

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/carlakc/boltnd/offersrpc"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lntest"
	"github.com/stretchr/testify/require"
)

// assertBlindedPathEqual asserts that two blinded paths are equal.
func assertBlindedPathEqual(t *testing.T, expected,
	actual *offersrpc.BlindedPath) {

	require.Equal(t, expected.IntroductionNode, actual.IntroductionNode,
		"introduction")

	require.Equal(t, expected.BlindingPoint, actual.BlindingPoint,
		"blinding point")

	require.Equal(t, len(expected.Hops), len(actual.Hops), "hop count")

	for i, hop := range expected.Hops {
		require.Equal(t, hop.BlindedNodeId,
			actual.Hops[i].BlindedNodeId, "blinded node id", i)

		require.Equal(t, hop.EncryptedData,
			actual.Hops[i].EncryptedData, "encrypted data", i)
	}
}

// consumeOnionMessage sets up a closure that can be used to consume messages
// delivered from an onion message subscription client.
//
// This function calls Recv() in a goroutine so that tests will not block in
// the case where we expect to receive a message but one does not arrive. An
// alternative here would be to use a context with timeout on the top level
// subscription, but this would require callers to know how long the test will
// take to run (or overestimate it).
func consumeOnionMessage(wg *sync.WaitGroup,
	msgChan chan *offersrpc.SubscribeOnionPayloadResponse,
	errChan chan error) func(client offersrpc.Offers_SubscribeOnionPayloadClient) {

	return func(client offersrpc.Offers_SubscribeOnionPayloadClient) {
		wg.Add(1)
		go func() {
			defer wg.Done()

			msg, err := client.Recv()
			if err != nil {
				errChan <- err
				return
			}

			msgChan <- msg
		}()
	}
}

// readOnionMessage sets up a closure that will read responses from our onion
// message subscription channels or fail after a timeout.
func readOnionMessage(msgChan chan *offersrpc.SubscribeOnionPayloadResponse,
	errChan chan error) func() (*offersrpc.SubscribeOnionPayloadResponse,
	error) {

	return func() (*offersrpc.SubscribeOnionPayloadResponse, error) {
		select {
		// If we receive a message as expected, assert that it is of
		// the correct type.
		case msg := <-msgChan:
			return msg, nil

		// If we received an error, something went wrong.
		case err := <-errChan:
			return nil, err

		// In the case of a timeout, let our test exit. This will
		// cancel the receive goroutine (through context cancelation)
		// and wait for it to exit.
		//
		// We allow messages up to a minute to arrive because lnd uses
		// low priority for custom messages, so our messages may be
		// queued for a while before they arrive. Increasing this to
		// a minute decreases test flakes significantly.
		case <-time.After(time.Minute):
			return nil, errors.New("message read timeout")
		}
	}
}

// openChannelAndAnnounce opens a channel from initiator -> receiver, fully
// confirming it and waiting until the initiator, recipient and optional set of
// nodes in the network slice have seen the channel announcement.
func openChannelAndAnnounce(t *testing.T, net *lntest.NetworkHarness,
	initiator, receiver *lntest.HarnessNode,
	network ...*lntest.HarnessNode) {

	chanReq := lntest.OpenChannelParams{
		Amt: 500_0000,
	}

	chanUpdates, err := net.OpenChannel(initiator, receiver, chanReq)
	require.NoError(t, err, "open channel")

	// Mine 6 blocks so that our channel will confirm.
	_, err = net.Miner.Client.Generate(6)
	require.NoError(t, err, "mine blocks")

	channelID, err := net.WaitForChannelOpen(chanUpdates)
	require.NoError(t, err, "chan open")

	// Wait for all nodes to see the channel between Alice and Bob.
	require.NoError(
		t, initiator.WaitForNetworkChannelOpen(channelID), "initiator",
	)
	require.NoError(
		t, receiver.WaitForNetworkChannelOpen(channelID), "receiver",
	)

	for _, node := range network {
		require.NoError(
			t, node.WaitForNetworkChannelOpen(channelID),
			"listener: %v", node.Name(),
		)
	}
}

// fundNode funds a node with 1BTC and waits for the balance to reflect in
// its confirmed wallet balance.
func fundNode(ctx context.Context, t *testing.T, net *lntest.NetworkHarness,
	node *lntest.HarnessNode) {

	ctxt, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	walletResp, err := node.WalletBalance(
		ctxt, &lnrpc.WalletBalanceRequest{},
	)
	require.NoError(t, err, "wallet balance")

	startBalance := walletResp.ConfirmedBalance

	addrReq := &lnrpc.NewAddressRequest{
		Type: lnrpc.AddressType_TAPROOT_PUBKEY,
	}

	ctxt, cancel = context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	resp, err := node.NewAddress(ctx, addrReq)
	require.NoError(t, err, "new address")

	addr, err := btcutil.DecodeAddress(resp.Address, node.Cfg.NetParams)
	require.NoError(t, err, "decode addr")

	addrScript, err := txscript.PayToAddrScript(addr)
	require.NoError(t, err, "pay to addr")

	output := &wire.TxOut{
		PkScript: addrScript,
		Value:    btcutil.SatoshiPerBitcoin,
	}

	_, err = net.Miner.SendOutputs([]*wire.TxOut{output}, 7500)
	require.NoError(t, err, "send outputs")

	_, err = net.Miner.Client.Generate(6)
	require.NoError(t, err, "generate")

	require.Eventually(t, func() bool {
		ctxt, cancel = context.WithTimeout(ctx, defaultTimeout)
		defer cancel()

		walletResp, err = node.WalletBalance(
			ctx, &lnrpc.WalletBalanceRequest{},
		)
		require.NoError(t, err, "wallet balance")

		// We do a loose check so that we don't have to worry about
		// fees etc.
		return walletResp.ConfirmedBalance > startBalance
	}, defaultTimeout, time.Second)
}

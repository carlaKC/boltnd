package onionmsg

import (
	"errors"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/carlakc/boltnd/testutils"
	"github.com/lightninglabs/lndclient"
	"github.com/lightningnetwork/lnd/keychain"
	"github.com/stretchr/testify/require"
)

// TestNodeECDH tests using lnd's apis to implement the SingleKeyECDH interface.
func TestNodeECDH(t *testing.T) {
	// Create a lnd mock, we'll use it for both of the dependencies required
	// for the nodeECDH implementation.
	lnd := testutils.NewMockLnd()
	defer lnd.Mock.AssertExpectations(t)

	// First, we'll test the case where we can't call GetInfo on startup
	// to get our node's pubkey.
	infoErr := errors.New("getinfo failed")
	testutils.MockGetInfo(lnd.Mock, &lndclient.Info{}, infoErr)

	nodeECDH, err := NewNodeECDH(lnd, lnd)
	require.True(t, errors.Is(err, infoErr), "getinfo error")
	require.Nil(t, nodeECDH, "node ecdh non-nil")

	// Now we'll test the (highly unlikely) case where we get an invalid
	// pubkey from lnd.
	info := &lndclient.Info{
		IdentityPubkey: [33]byte{
			// Valid compressed pubkeys begin with 0x02 or 0x03,
			// create an invalid pubkey with 0x04.
			0x04,
		},
	}

	testutils.MockGetInfo(lnd.Mock, info, nil)
	nodeECDH, err = NewNodeECDH(lnd, lnd)

	// Our pubkey parsing does not wrap errors, do the best we can do here
	// is string match.
	require.NotNil(t, err, "expected pubkey failure")
	require.Contains(t, err.Error(), "unsupported format")
	require.Nil(t, nodeECDH, "node ecdh non-nil")

	// Now, test the case where we can successfully call GetInfo and create
	// our nodeECDH instance.
	pubkeys := testutils.GetPubkeys(t, 2)
	copy(info.IdentityPubkey[:], pubkeys[0].SerializeCompressed())
	testutils.MockGetInfo(lnd.Mock, info, nil)

	nodeECDH, err = NewNodeECDH(lnd, lnd)
	require.NoError(t, err, "successful getinfo")
	require.NotNil(t, nodeECDH, "node ecdh nil")

	// We should have stored our pubkey, so calls to Pubkey() don't trigger
	// any lnd api calls.
	nodePubkey, err := btcec.ParsePubKey(pubkeys[0].SerializeCompressed())
	require.NoError(t, err, "invalid pubkey")

	require.Equal(t, nodePubkey, nodeECDH.PubKey())

	// Setup expected parameters and return for our mock.
	nodeKeyLoc := &keychain.KeyLocator{
		Family: keychain.KeyFamilyNodeKey,
		Index:  0,
	}
	ephemeral := pubkeys[1]
	expectedECDH := [32]byte{1, 2, 3}

	testutils.MockDeriveSharedKey(
		lnd.Mock, ephemeral, nodeKeyLoc, expectedECDH, nil,
	)

	// Finally, test performing ECDH operations via an API call.
	actualECDH, err := nodeECDH.ECDH(ephemeral)
	require.NoError(t, err, "ecdh failed")
	require.Equal(t, expectedECDH, actualECDH, "ecdh result")
}

package testutils

import (
	"encoding/hex"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/stretchr/testify/require"
)

// GetPubkeys provides n public keys for testing.
func GetPubkeys(t *testing.T, n int) []*btcec.PublicKey {
	t.Helper()

	if len(pubkeyStrs) < n {
		t.Fatalf("testing package only has: %v pubkeys, %v requested",
			len(pubkeyStrs), n)
	}

	pubkeys := make([]*btcec.PublicKey, n)

	for i, pkStr := range pubkeyStrs[0:n] {
		pkBytes, err := hex.DecodeString(pkStr)
		require.NoError(t, err, "pubkey decode string")

		pubkeys[i], err = btcec.ParsePubKey(pkBytes)
		require.NoError(t, err, "parse pubkey")
	}

	return pubkeys
}

// GetPrivkeys provides n private keys for testing.
func GetPrivkeys(t *testing.T, n int) []*btcec.PrivateKey {
	t.Helper()

	if len(privkeyStrs) < n {
		t.Fatalf("testing package only has: %v privkeys, %v requested",
			len(privkeyStrs), n)
	}

	privkeys := make([]*btcec.PrivateKey, n)

	for i, pkStr := range privkeyStrs[0:n] {
		pkBytes, err := hex.DecodeString(pkStr)
		require.NoError(t, err, "privkey decode string")

		privkeys[i], _ = btcec.PrivKeyFromBytes(pkBytes)
		require.NoError(t, err, "parse privkey")
	}

	return privkeys
}

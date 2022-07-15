package onionmsg

import (
	"context"
	"fmt"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	sphinx "github.com/lightningnetwork/lightning-onion"
	"github.com/lightningnetwork/lnd/keychain"
)

var defaultTimeout = time.Second * 15

// NodeECDH provides the functionality required for an onion router's
// cryptographic operations, specifically ECDH operations with the node's
// pubkey, using lnd's public apis to perform the operations.
type NodeECDH struct {
	signer  LndOnionSigner
	pubkey  *btcec.PublicKey
	timeout time.Duration
}

// NewNodeECDH creates a new node key ecdh which uses lnd's external apis to
// perform the operations required.
func NewNodeECDH(lnd LndOnionMsg, signer LndOnionSigner) (*NodeECDH,
	error) {

	r := &NodeECDH{
		signer:  signer,
		timeout: defaultTimeout,
	}

	// Perform a once-off pubkey fetch so that we don't need to hit
	// lnd's API every time we need our pubkey.
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	info, err := lnd.GetInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("get info failed: %w", err)
	}

	r.pubkey, err = btcec.ParsePubKey(info.IdentityPubkey[:])
	if err != nil {
		return nil, fmt.Errorf("could not parse node key: %w", err)
	}

	return r, nil
}

// Compile time assertion that NodeECDH implements the SingleKeyECDH interface.
var _ sphinx.SingleKeyECDH = (*NodeECDH)(nil)

// PubKey returns the lnd node's public key.
func (r *NodeECDH) PubKey() *btcec.PublicKey {
	return r.pubkey
}

// ECDH performs ECDH operations with the node's public key, returning the
// SHA256 hash of the shared secret.
func (r *NodeECDH) ECDH(pubkey *btcec.PublicKey) ([32]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	// Point lnd to our node's key for EDCH operations.
	keyLoc := &keychain.KeyLocator{
		Family: keychain.KeyFamilyNodeKey,
		Index:  0,
	}

	return r.signer.DeriveSharedKey(ctx, pubkey, keyLoc)
}

package routes

import (
	"github.com/btcsuite/btcd/btcec/v2"
)

// BlindedRouteGenerator produces blinded routes.
type BlindedRouteGenerator struct {
	// lnd provides access to our lnd node.
	lnd Lnd

	// pubkey is our node's public key.
	pubkey *btcec.PublicKey
}

// NewBlindedRouteGenerator creates a blinded route generator.
func NewBlindedRouteGenerator(lnd Lnd,
	pubkey *btcec.PublicKey) *BlindedRouteGenerator {

	return &BlindedRouteGenerator{
		lnd:    lnd,
		pubkey: pubkey,
	}
}

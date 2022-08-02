package lnwire

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
)

var (
	// lightningTag is the top level tag used to tag signatures on offers.
	lightningTag = []byte("lightning")

	// offerTag is the message tag used to tag signatures on offers.
	offerTag = []byte("offer")

	// invoiceTag is the message tag used to tag signatures on invoices.
	invoiceTag = []byte("invoice")

	// signatureTag is the field tag used to tag signatures (TLV type= 240)
	// for offers.
	signatureTag = []byte("signature")
)

// signatureDigest returns the tagged merkle root that is used for offer
// signatures.
func signatureDigest(messageTag, fieldTag []byte,
	root chainhash.Hash) chainhash.Hash {

	// The tag has the following format:
	// lightning || message tag || field tag
	tags := [][]byte{
		lightningTag, messageTag, fieldTag,
	}

	// Create a tagged hash with the merkle root.
	digest := chainhash.TaggedHash(
		bytes.Join(tags, []byte{}), root[:],
	)

	return *digest
}

func validateSignature(signature [64]byte, nodeID *btcec.PublicKey,
	digest []byte) error {

	sig, err := schnorr.ParseSignature(signature[:])
	if err != nil {
		return fmt.Errorf("invalid signature: %v: %w",
			hex.EncodeToString(signature[:]), err)
	}

	if !sig.Verify(digest, nodeID) {
		return fmt.Errorf("%w: %v for: %v from: %v", ErrInvalidOfferSig,
			hex.EncodeToString(signature[:]),
			hex.EncodeToString(digest),
			hex.EncodeToString(schnorr.SerializePubKey(nodeID)),
		)
	}

	return nil
}

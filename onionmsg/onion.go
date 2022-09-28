package onionmsg

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/carlakc/boltnd/lnwire"
	"github.com/lightninglabs/lndclient"
	sphinx "github.com/lightningnetwork/lightning-onion"
	"github.com/lightningnetwork/lnd/routing/route"
)

var (
	// ErrNoEncryptedData is returned when the encrypted data TLV is not
	// present when it is required.
	ErrNoEncryptedData = errors.New("encrypted data blob required")

	// ErrNoForwardingPayload is returned when no onion message payload
	// is provided to allow forwarding messages.
	ErrNoForwardingPayload = errors.New("no payload provided for " +
		"forwarding")
)

// customOnionMessage encodes the onion message provided and wraps it in a
// lnd custom message so that it can be sent to peers via external apis.
func customOnionMessage(peer *btcec.PublicKey,
	msg *lnwire.OnionMessage) (*lndclient.CustomMessage, error) {

	buf := new(bytes.Buffer)
	if err := msg.Encode(buf, 0); err != nil {
		return nil, fmt.Errorf("onion message encode: %w", err)
	}

	return &lndclient.CustomMessage{
		Peer:    route.NewVertex(peer),
		MsgType: lnwire.OnionMessageType,
		Data:    buf.Bytes(),
	}, nil
}

// decryptBlobFunc returns a closure that can be used to decrypt an onion
// message's encrypted data blob and decode it.
func decryptBlobFunc(nodeKey sphinx.SingleKeyECDH) func(*btcec.PublicKey,
	*lnwire.OnionMessagePayload) (*lnwire.BlindedRouteData, error) {

	return func(blindingPoint *btcec.PublicKey,
		payload *lnwire.OnionMessagePayload) (*lnwire.BlindedRouteData,
		error) {

		if payload == nil {
			return nil, ErrNoForwardingPayload
		}

		if len(payload.EncryptedData) == 0 {
			return nil, ErrNoEncryptedData
		}

		decrypted, err := sphinx.DecryptBlindedData(
			nodeKey, blindingPoint, payload.EncryptedData,
		)
		if err != nil {
			return nil, fmt.Errorf("could not decrypt data "+
				"blob: %w", err)
		}

		data, err := lnwire.DecodeBlindedRouteData(decrypted)
		if err != nil {
			return nil, fmt.Errorf("could not decode data "+
				"blob: %w", err)
		}

		return data, nil
	}
}

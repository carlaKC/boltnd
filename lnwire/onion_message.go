package lnwire

import (
	"bytes"
	"fmt"
	"io"

	"github.com/btcsuite/btcd/btcec/v2"
	lndwire "github.com/lightningnetwork/lnd/lnwire"
)

// OnionMessageType is the protocol message type used for onion messages in lnd.
const OnionMessageType = 513

// OnionMessage represents an onion message used to communicate with peers.
type OnionMessage struct {
	// BlindingPoint is the route blinding ephemeral pubkey to be used for
	// the onion message.
	BlindingPoint *btcec.PublicKey

	// OnionBlob is the raw serialized mix header used to relay messages in
	// a privacy-preserving manner. This blob should be handled in the same
	// manner as onions used to route HTLCs, with the exception that it uses
	// blinded routes by default.
	OnionBlob []byte
}

// NewOnionMessage creates a new onion message.
func NewOnionMessage(blindingPoint *btcec.PublicKey, onion []byte) *OnionMessage {
	return &OnionMessage{
		BlindingPoint: blindingPoint,
		OnionBlob:     onion,
	}
}

// A compile time check to ensure OnionMessage implements the lnwire.Message
// interface in lnd.
var _ lndwire.Message = (*OnionMessage)(nil)

// MsgType returns the integer uniquely identifying this message type on the
// wire.
//
// This is part of the lnwire.Message interface in lnd.
func (o *OnionMessage) MsgType() lndwire.MessageType {
	return lndwire.MessageType(OnionMessageType)
}

// Decode reads the bytes stream and converts it to the object.
//
// This is part of the lnwire.Message interface in lnd.
func (o *OnionMessage) Decode(r io.Reader, _ uint32) error {
	if err := lndwire.ReadElement(r, &o.BlindingPoint); err != nil {
		return fmt.Errorf("decode blinding point: %w", err)
	}

	var onionLen uint16
	if err := lndwire.ReadElement(r, &onionLen); err != nil {
		return fmt.Errorf("decode onion len: %w", err)
	}

	o.OnionBlob = make([]byte, onionLen)
	if err := lndwire.ReadElement(r, o.OnionBlob); err != nil {
		return fmt.Errorf("read onion blob: %w", err)
	}

	return nil
}

// Encode converts object to the bytes stream and write it into the
// write buffer.
//
// This is part of the lnwire.Message interface in lnd.
func (o *OnionMessage) Encode(w *bytes.Buffer, _ uint32) error {
	if err := lndwire.WriteElement(w, o.BlindingPoint); err != nil {
		return fmt.Errorf("encode blinding point: %w", err)
	}

	onionLen := len(o.OnionBlob)
	if err := lndwire.WriteElement(w, uint16(onionLen)); err != nil {
		return fmt.Errorf("encode onion len: %w", err)
	}

	if err := lndwire.WriteElement(w, o.OnionBlob); err != nil {
		return fmt.Errorf("encode onion blob: %w", err)
	}

	return nil
}

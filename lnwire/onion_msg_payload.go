package lnwire

import (
	"fmt"
	"io"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/lightningnetwork/lnd/tlv"
)

// BlindedHop contains a blinded node ID and encrypted data used to send onion
// messages over blinded routes.
type BlindedHop struct {
	// BlindedNodeID is the blinded node id of a node in the path.
	BlindedNodeID *btcec.PublicKey

	// EncryptedData is the encrypted data to be included for the node.
	EncryptedData []byte
}

// size returns the encoded size of a blinded hop.
func (b *BlindedHop) size() uint64 {
	// 33 byte pubkey + 2 bytes uint16 length + var bytes.
	return uint64(33 + 2 + len(b.EncryptedData))
}

// encodeBlindedHop encodes a blinded hop tlv.
func encodeBlindedHop(w io.Writer, val interface{}, buf *[8]byte) error {
	if b, ok := val.(*BlindedHop); ok {
		if err := tlv.EPubKey(w, &b.BlindedNodeID, buf); err != nil {
			return fmt.Errorf("encode blinded id: %w", err)
		}

		dataLen := uint16(len(b.EncryptedData))
		if err := tlv.EUint16(w, &dataLen, buf); err != nil {
			return fmt.Errorf("data len: %w", err)
		}

		if err := tlv.EVarBytes(w, &b.EncryptedData, buf); err != nil {
			return fmt.Errorf("encode encrypted data: %w", err)
		}

		return nil
	}

	return tlv.NewTypeForEncodingErr(val, "*BlindedHop")
}

// decodeBlindedHop decodes a blinded hop tlv, returning the number of bytes
// that it has read. This value can be used while decoding multiple variable
// length hops to determine whether there are more hops left.
func decodeBlindedHop(r io.Reader, val interface{}, buf *[8]byte,
	l uint64) (uint64, error) {

	// We know that our length should be _at least_ 33 bytes for a pubkey
	// and 2 bytes for our data length. We allow exactly 32 bytes to
	// handle the case where our data length is 0, and we don't have any
	// encrypted data included.
	if b, ok := val.(*BlindedHop); ok && l >= 35 {
		err := tlv.DPubKey(r, &b.BlindedNodeID, buf, 33)
		if err != nil {
			return 0, fmt.Errorf("decode blinded id: %w", err)
		}

		var dataLen uint16
		err = tlv.DUint16(r, &dataLen, buf, 2)
		if err != nil {
			return 0, fmt.Errorf("decode data len: %w", err)
		}

		err = tlv.DVarBytes(r, &b.EncryptedData, buf, uint64(dataLen))
		if err != nil {
			return 0, fmt.Errorf("decode data: %w", err)
		}

		// Return the total number of bytes read from the buffer.
		bytesRead := 33 + 2 + uint64(dataLen)
		return bytesRead, nil
	}

	return 0, tlv.NewTypeForDecodingErr(val, "*BlindedHop", l, l)
}

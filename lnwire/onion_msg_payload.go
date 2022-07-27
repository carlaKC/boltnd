package lnwire

import (
	"bytes"
	"fmt"
	"io"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/lightningnetwork/lnd/tlv"
)

const (
	// replyPathType is a record for onion messaging reply paths.
	replyPathType tlv.Type = 2

	// encryptedDataTLVType is a record containing encrypted data for
	// message recipient.
	encryptedDataTLVType tlv.Type = 4
)

// OnionMessagePayload contains the contents of an onion message payload.
type OnionMessagePayload struct {
	// ReplyPath contains a blinded path that can be used to respond to an
	// onion message.
	ReplyPath *ReplyPath

	// EncryptedData contains enrypted data for the recipient.
	EncryptedData []byte
}

// EncodeOnionMessagePayload encodes an onion message's final payload.
func EncodeOnionMessagePayload(o *OnionMessagePayload) ([]byte, error) {
	var records []tlv.Record

	if o.ReplyPath != nil {
		records = append(records, o.ReplyPath.record())
	}

	if len(o.EncryptedData) != 0 {
		record := tlv.MakePrimitiveRecord(
			encryptedDataTLVType, &o.EncryptedData,
		)
		records = append(records, record)
	}

	stream, err := tlv.NewStream(records...)
	if err != nil {
		return nil, fmt.Errorf("new stream: %w", err)
	}

	b := new(bytes.Buffer)
	if err := stream.Encode(b); err != nil {
		return nil, fmt.Errorf("encode stream: %w", err)
	}

	return b.Bytes(), nil
}

// DecodeOnionMessagePayload decodes an onion message's payload.
func DecodeOnionMessagePayload(o []byte) (*OnionMessagePayload, error) {
	onionPayload := &OnionMessagePayload{
		// Create a non-nil entry so that we can directly decode into
		// it.
		ReplyPath: &ReplyPath{},
	}

	records := []tlv.Record{
		onionPayload.ReplyPath.record(),
		tlv.MakePrimitiveRecord(
			encryptedDataTLVType, &onionPayload.EncryptedData,
		),
	}

	stream, err := tlv.NewStream(records...)
	if err != nil {
		return nil, fmt.Errorf("new stream: %w", err)
	}

	r := bytes.NewReader(o)
	tlvMap, err := stream.DecodeWithParsedTypes(r)
	if err != nil {
		return nil, fmt.Errorf("decode stream: %w", err)
	}

	// If our reply path wasn't populated, replace it with a nil entry.
	if _, ok := tlvMap[replyPathType]; !ok {
		onionPayload.ReplyPath = nil
	}

	return onionPayload, nil
}

// ReplyPath is a blinded path used to respond to onion messages.
type ReplyPath struct {
	// FirstNodeID is the pubkey of the first node in the reply path.
	FirstNodeID *btcec.PublicKey

	// BlindingPoint is the ephemeral pubkey used in route blinding.
	BlindingPoint *btcec.PublicKey

	// Hops is a set of blinded hops following the first node ID to deliver
	// responses to.
	Hops []*BlindedHop
}

// record produces a tlv record for a reply path.
func (r *ReplyPath) record() tlv.Record {
	return tlv.MakeDynamicRecord(
		replyPathType, r, r.size, encodeReplyPath, decodeReplyPath,
	)
}

// size returns the encoded size of our reply path.
func (r *ReplyPath) size() uint64 {
	// First node pubkey 33 + blinding point pubkey 33.
	size := uint64(33 + 33)

	// Add each hop's size to our total.
	for _, hop := range r.Hops {
		size += hop.size()
	}

	return size
}

// encodeReplyPath encodes a reply path tlv.
func encodeReplyPath(w io.Writer, val interface{}, buf *[8]byte) error {
	if p, ok := val.(*ReplyPath); ok {
		if err := tlv.EPubKey(w, &p.FirstNodeID, buf); err != nil {
			return fmt.Errorf("encode first node id: %w", err)
		}

		if err := tlv.EPubKey(w, &p.BlindingPoint, buf); err != nil {
			return fmt.Errorf("encode blinded path: %w", err)
		}

		for i, hop := range p.Hops {
			if err := encodeBlindedHop(w, hop, buf); err != nil {
				return fmt.Errorf("hop %v: %w", i, err)
			}
		}

		return nil
	}

	return tlv.NewTypeForEncodingErr(val, "*ReplyPath")
}

// decodeReplyPath decodes a reply path tlv.
func decodeReplyPath(r io.Reader, val interface{}, buf *[8]byte,
	l uint64) error {

	if p, ok := val.(*ReplyPath); ok && l > 35 {
		err := tlv.DPubKey(r, &p.FirstNodeID, buf, 33)
		if err != nil {
			return fmt.Errorf("decode first id: %w", err)
		}

		err = tlv.DPubKey(r, &p.BlindingPoint, buf, 33)
		if err != nil {
			return fmt.Errorf("decode blinding point:  %w", err)
		}

		// Track the number of bytes that we expect to have left in
		// our record.
		remainingBytes := l - 33 - 33

		// We expect the remainder of bytes in this message to contain
		// blinded hops. Decode hops and add them to our path until
		// we've read the full record. If we have a partial number of
		// bytes left (ie, not enough to decode a full hop), we expect
		// decodeBlindedHop to fail, so we don't need to check that we
		// exactly hit 0 remaining bytes.
		for remainingBytes > 0 {
			hop := &BlindedHop{}
			bytesRead, err := decodeBlindedHop(
				r, hop, buf, uint64(remainingBytes),
			)
			if err != nil {
				return fmt.Errorf("decode hop: %w", err)
			}

			p.Hops = append(p.Hops, hop)
			remainingBytes -= bytesRead
		}

		return nil
	}

	return tlv.NewTypeForDecodingErr(val, "*ReplyPath", l, l)
}

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

package lnwire

import (
	"io"

	"github.com/lightningnetwork/lnd/tlv"
)

// encodeTU64 encodes a truncated uint64 tlv.
//
// Note: lnd doesn't have this functionality on its own yet (only in mpp encode)
// so it is added here.
func encodeTU64(w io.Writer, val interface{}, buf *[8]byte) error {
	if v, ok := val.(*uint64); ok {
		return tlv.ETUint64T(w, *v, buf)
	}

	return tlv.NewTypeForEncodingErr(val, "tu64")
}

// decodeTU64 decodes a truncated uint64 tlv.
//
// Note: lnd doesn't have this functionality on its own yet (only in mpp decode)
// so it is added here.
func decodeTU64(r io.Reader, val interface{}, buf *[8]byte, l uint64) error {
	if v, ok := val.(*uint64); ok && 1 <= l && l <= 8 {
		if err := tlv.DTUint64(r, v, buf, l); err != nil {
			return err
		}

		return nil
	}

	return tlv.NewTypeForDecodingErr(val, "tu64", l, l)
}

func tu64Record(tlvType tlv.Type, value *uint64) tlv.Record {
	return tlv.MakeDynamicRecord(tlvType, value, func() uint64 {
		return tlv.SizeTUint64(*value)
	}, encodeTU64, decodeTU64)
}

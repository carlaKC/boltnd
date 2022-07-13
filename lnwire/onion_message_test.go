package lnwire

import (
	"bytes"
	"testing"

	"github.com/carlakc/boltnd/testutils"
	"github.com/stretchr/testify/require"
)

// TestOnionMessageEncode tests encoding and decoding of onion messages.
func TestOnionMessageEncode(t *testing.T) {
	pubkeys := testutils.GetPubkeys(t, 1)
	blob := []byte{1, 2, 3}

	expected := NewOnionMessage(pubkeys[0], blob)

	buf := new(bytes.Buffer)
	err := expected.Encode(buf, 0)
	require.NoError(t, err, "encode onion")

	actual := &OnionMessage{}
	err = actual.Decode(buf, 0)
	require.NoError(t, err, "onion decode")
	require.Equal(t, expected, actual, "message comparison")
}

package onionmsg

import (
	"errors"
	"testing"

	"github.com/carlakc/boltnd/lnwire"
	"github.com/carlakc/boltnd/testutils"
	sphinx "github.com/lightningnetwork/lightning-onion"
	"github.com/stretchr/testify/require"
)

// TestDecryptBlob tests decrypting of onion message blobs.
func TestDecryptBlob(t *testing.T) {
	var (
		privkeys = testutils.GetPrivkeys(t, 3)
		nodeECDH = &sphinx.PrivKeyECDH{
			PrivKey: privkeys[0],
		}

		blindingPrivkey = privkeys[1]
	)

	tests := []struct {
		name    string
		payload *lnwire.OnionMessagePayload
		err     error
	}{
		{
			name:    "no payload",
			payload: nil,
			err:     ErrNoForwardingPayload,
		},
		{
			name: "no encrypted data",
			payload: &lnwire.OnionMessagePayload{
				EncryptedData: nil,
			},
			err: ErrNoEncryptedData,
		},
	}

	for _, testCase := range tests {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			decryptBlob := decryptBlobFunc(nodeECDH)

			_, err := decryptBlob(
				blindingPrivkey.PubKey(), testCase.payload,
			)
			require.True(t, errors.Is(err, testCase.err))
		})
	}
}

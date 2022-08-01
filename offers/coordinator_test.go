package offers

import (
	"errors"
	"testing"

	"github.com/carlakc/boltnd/lnwire"
	"github.com/carlakc/boltnd/testutils"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/stretchr/testify/require"
)

// TestCoordinatorLifecycle tests starting and shutdown of the coordinator.
func TestCoordinatorLifecycle(t *testing.T) {
	// Creating a test case and starting/stopping it will assert that we
	// shutdown on stop with no errors.
	coordinatorTest := newOfferCoordinatorTest(t)
	coordinatorTest.start()
	coordinatorTest.stop()
}

// TestValidateExchange tests validation of an invoice against the offer and
// request that it is associated with.
func TestValidateExchange(t *testing.T) {
	var (
		id1Bytes       = [32]byte{1}
		id2Bytes       = [32]byte{2}
		chainHashBytes = [32]byte{3}

		pubkeys = testutils.GetPubkeys(t, 2)
	)

	id1, err := lntypes.MakeHash(id1Bytes[:])
	require.NoError(t, err)

	id2, err := lntypes.MakeHash(id2Bytes[:])
	require.NoError(t, err)

	chainHash, err := lntypes.MakeHash(chainHashBytes[:])
	require.NoError(t, err)

	okOffer := &lnwire.Offer{
		MerkleRoot:    id1,
		NodeID:        pubkeys[0],
		Description:   "offer description",
		MinimumAmount: 100,
	}

	tests := []struct {
		name           string
		offer          *lnwire.Offer
		invoiceRequest *lnwire.InvoiceRequest
		invoice        *lnwire.Invoice
		err            error
	}{
		{
			name: "chainhash mismatch - request",
			offer: &lnwire.Offer{
				Chainhash: chainHash,
			},
			invoiceRequest: &lnwire.InvoiceRequest{
				Chainhash: id1,
			},
			err: ErrChainhashMismatch,
		},
		{
			name: "chainhash mismatch - invoice",
			offer: &lnwire.Offer{
				Chainhash: chainHash,
			},
			invoiceRequest: &lnwire.InvoiceRequest{
				Chainhash: chainHash,
			},
			invoice: &lnwire.Invoice{
				Chainhash: id1,
			},
			err: ErrChainhashMismatch,
		},
		{
			name: "offer id mismatch",
			offer: &lnwire.Offer{
				MerkleRoot: id1,
			},
			invoiceRequest: &lnwire.InvoiceRequest{},
			invoice: &lnwire.Invoice{
				OfferID: id2,
			},
			err: ErrOfferIDIncorrect,
		},
		{
			name: "node id mismatch",
			offer: &lnwire.Offer{
				MerkleRoot: id1,
				NodeID:     pubkeys[0],
			},
			invoiceRequest: &lnwire.InvoiceRequest{},
			invoice: &lnwire.Invoice{
				OfferID: id1,
				NodeID:  pubkeys[1],
			},
			err: ErrNodeIDIncorrect,
		},
		{
			name: "description mismatch",
			offer: &lnwire.Offer{
				MerkleRoot:  id1,
				NodeID:      pubkeys[0],
				Description: "offer description",
			},
			invoiceRequest: &lnwire.InvoiceRequest{},
			invoice: &lnwire.Invoice{
				OfferID:     id1,
				NodeID:      pubkeys[0],
				Description: "invoice description",
			},
			err: ErrDescriptionIncorrect,
		},
		{
			name: "amount wrong",
			offer: &lnwire.Offer{
				MerkleRoot:    id1,
				NodeID:        pubkeys[0],
				Description:   "offer description",
				MinimumAmount: 100,
			},
			invoiceRequest: &lnwire.InvoiceRequest{},
			invoice: &lnwire.Invoice{
				OfferID:     id1,
				NodeID:      pubkeys[0],
				Description: "offer description",
				Amount:      80,
			},
			err: ErrAmountIncorrect,
		},
		{
			name:  "payer key mismatch",
			offer: okOffer,
			invoice: &lnwire.Invoice{
				OfferID:     okOffer.MerkleRoot,
				NodeID:      okOffer.NodeID,
				Description: okOffer.Description,
				Amount:      okOffer.MinimumAmount,
				PayerKey:    pubkeys[0],
			},
			invoiceRequest: &lnwire.InvoiceRequest{
				PayerKey: pubkeys[1],
			},
			err: ErrPayerKeyMismatch,
		},
		{
			name:  "payer info mismatch",
			offer: okOffer,
			invoice: &lnwire.Invoice{
				OfferID:     okOffer.MerkleRoot,
				NodeID:      okOffer.NodeID,
				Description: okOffer.Description,
				Amount:      okOffer.MinimumAmount,
				PayerKey:    pubkeys[0],
				PayerInfo:   []byte{1},
			},
			invoiceRequest: &lnwire.InvoiceRequest{
				PayerKey:  pubkeys[0],
				PayerInfo: []byte{2},
			},
			err: ErrPayerInfoMismatch,
		},
		{
			name:  "payer info mismatch",
			offer: okOffer,
			invoice: &lnwire.Invoice{
				OfferID:     okOffer.MerkleRoot,
				NodeID:      okOffer.NodeID,
				Description: okOffer.Description,
				Amount:      okOffer.MinimumAmount,
				PayerKey:    pubkeys[0],
				PayerInfo:   []byte{1},
				PayerNote:   "payer note",
			},
			invoiceRequest: &lnwire.InvoiceRequest{
				PayerKey:  pubkeys[0],
				PayerInfo: []byte{1},
				PayerNote: "different payer note",
			},
			err: ErrPayerNoteMismatch,
		},
		{
			name:  "quantity mismatch",
			offer: okOffer,
			invoice: &lnwire.Invoice{
				OfferID:     okOffer.MerkleRoot,
				NodeID:      okOffer.NodeID,
				Description: okOffer.Description,
				Amount:      okOffer.MinimumAmount,
				PayerKey:    pubkeys[0],
				PayerInfo:   []byte{1},
				PayerNote:   "payer note",
				Quantity:    12,
			},
			invoiceRequest: &lnwire.InvoiceRequest{
				PayerKey:  pubkeys[0],
				PayerInfo: []byte{1},
				PayerNote: "payer note",
				Quantity:  10,
			},
			err: ErrQuantityMismatch,
		},
		{
			name:  "valid",
			offer: okOffer,
			invoice: &lnwire.Invoice{
				OfferID:     okOffer.MerkleRoot,
				NodeID:      okOffer.NodeID,
				Description: okOffer.Description,
				Amount:      okOffer.MinimumAmount,
				PayerKey:    pubkeys[0],
				PayerInfo:   []byte{1},
				PayerNote:   "payer note",
				Quantity:    12,
			},
			invoiceRequest: &lnwire.InvoiceRequest{
				PayerKey:  pubkeys[0],
				PayerInfo: []byte{1},
				PayerNote: "payer note",
				Quantity:  12,
			},
			err: nil,
		},
	}

	for _, testCase := range tests {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			err := validateExchange(
				testCase.offer, testCase.invoiceRequest,
				testCase.invoice,
			)

			require.True(t, errors.Is(err, testCase.err))
		})
	}
}

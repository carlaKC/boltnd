package itest

import (
	"context"
	"testing"

	"github.com/carlakc/boltnd/offersrpc"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// offerStr contains an encoded, valid offer.
	offerStr = "lno1pqqnyzsmx5cx6umpwssx6atvw35j6ut4v9h8g6t50ysx7enxv4" +
		"epgrmjw4ehgcm0wfczucm0d5hxzagkqyq3ugztng063cqx783exlm97ek" +
		"yprnd4rsu5u5w5sez9fecrhcuc3ykq5"

	// nodeIDStr is the node id encoded in the offer string.
	nodeIDStr = "4b9a1fa8e006f1e3937f65f66c408e6da8e1ca728ea43222a7381d" +
		"f1cc449605"

	// signedOfferStr contains an encoded, signed, valid offer.
	signedOfferStr = "lno1pg257enxv4ezqcneype82um50ynhxgrwdajx283qfwdp" +
		"l28qqmc78ymlvhmxcsywdk5wrjnj36jryg488qwlrnzyjczlqs85ck65y" +
		"cmkdk92smwt9zuewdzfe7v4aavvaz5kgv9mkk63v3s0ge0f099kssh3yc" +
		"95qztx504hu92hnx8ctzhtt08pgk0texz0509tk"
)

// DecodeOfferTestCase tests decoding of offer strings.
func DecodeOfferTestCase(t *testing.T, ht *harnessTest) {
	offersTest := ht.bolt12

	ctxb := context.Background()
	ctxt, cancel := context.WithTimeout(ctxb, defaultTimeout)
	defer cancel()

	// First, test the case where we omit an offer string and assert that
	// we get an invalid argument error.
	req := &offersrpc.DecodeOfferRequest{}
	_, err := offersTest.aliceOffers.DecodeOffer(ctxt, req)
	require.Error(t, err, "expect error for empty request")

	status, ok := status.FromError(err)
	require.True(t, ok, "grpc error required")
	require.Equal(t, codes.InvalidArgument, status.Code(), err)

	// Next, test the case where we provide a valid offer string and
	// successfully decode it.
	req.Offer = offerStr

	resp, err := offersTest.aliceOffers.DecodeOffer(ctxt, req)
	require.NoError(t, err, "offer decode")

	// The values for our expected offer are obtained from:
	// https://bootstrap.bolt12.org/decode/{offerStr}
	//
	// Protos have some unexported fields that we can't set, so we check
	// each expected field in the offer.
	require.Equal(t, uint64(50), resp.Offer.MinAmountMsat, "min amount")
	require.Equal(t, "50msat multi-quantity offer", resp.Offer.Description)
	require.Equal(t, "rustcorp.com.au", resp.Offer.Issuer, "issuer")
	require.Equal(t, uint64(1), resp.Offer.MinQuantity, "min quantity")
	require.Equal(t, uint64(0), resp.Offer.MaxQuantity, "max quantity")
	require.Equal(t, nodeIDStr, resp.Offer.NodeId, "node id")
	require.Equal(t, "", resp.Offer.Signature, "signature")

	// Next, test decoding of a signed offer.
	req.Offer = signedOfferStr
	resp, err = offersTest.aliceOffers.DecodeOffer(ctxt, req)
	require.NoError(t, err, "signed offer decode")

	require.Equal(t, "Offer by rusty's node", resp.Offer.Description)
	require.Equal(t, nodeIDStr, resp.Offer.NodeId, "node id")
}

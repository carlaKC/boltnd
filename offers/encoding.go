package offers

import (
	"fmt"

	"github.com/btcsuite/btcd/btcutil/bech32"
)

// DecodeOfferStr decodes a bech32 encoded offer string, returning our offer
// type with the information contained in the offer.
func DecodeOfferStr(offerStr string) (*Offer, error) {
	// First, strip any joining characters / spare whitespace from the
	// offer.
	cleanOffer, err := stripOffer(offerStr)
	if err != nil {
		return nil, fmt.Errorf("strip offer: %w", err)
	}

	hrp, data, err := decodeBech32(cleanOffer)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidOfferStr, err)
	}

	if hrp != offerHRP {
		return nil, fmt.Errorf("%w: got: %v", ErrBadHRP, hrp)
	}

	offerBytes, err := bech32.ConvertBits(data, 5, 8, false)
	if err != nil {
		return nil, fmt.Errorf("convert bits: %w", err)
	}

	offer, err := DecodeOffer(offerBytes)
	if err != nil {
		return nil, fmt.Errorf("could not decode offer: %w", err)
	}

	if err := offer.Validate(); err != nil {
		return nil, fmt.Errorf("invalid offer: %w", err)
	}

	return offer, nil
}

package offers

import (
	"errors"
	"fmt"
	"strings"
)

const (
	charset  = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"
	offerHRP = "lno"
)

var (
	// ErrIncorrectSplit is returned when an offer string contains "+"
	// joins that are incorrectly placed - either consecutive, or starting/
	// ending the offers string
	ErrIncorrectSplit = errors.New("consecutive, prefix or suffix + invalid")

	// ErrNotInCharset is returned when a character in an offer string is
	// not part of our charset.
	ErrNotInCharset = errors.New("invalid character, not in charset")
)

// decodeBech32 decodes a bech32 encoded string, returning the human-readable
// part and the data. This function does not expect a checksum to be included.
//
// Note: the data will be base32 encoded, that is each element of the returned
// byte array will encode 5 bits of data. Use the ConvertBits method to convert
// this to 8-bit representation.
//
// Note: This code is copied from lnd/zpay32/bech32.go @ 7662ea5d4d, full
// credit to the LL developers. Checksum verification is removed because we do
// not need it for bolt 12.
func decodeBech32(bech string) (string, []byte, error) {
	// The maximum allowed length for a bech32 string is 90. It must also
	// be at least 8 characters, since it needs a non-empty HRP, a
	// separator, and a 6 character checksum.
	// NB: The 90 character check specified in BIP173 is skipped here, to
	// allow strings longer than 90 characters.
	if len(bech) < 8 {
		return "", nil, fmt.Errorf("invalid bech32 string length %d",
			len(bech))
	}

	// Only	ASCII characters between 33 and 126 are allowed.
	if err := checkASCII(bech); err != nil {
		return "", nil, err
	}

	// The characters must be either all lowercase or all uppercase.
	lower := strings.ToLower(bech)
	upper := strings.ToUpper(bech)
	if bech != lower && bech != upper {
		return "", nil, fmt.Errorf("string not all lowercase or all " +
			"uppercase")
	}

	// We'll work with the lowercase string from now on.
	bech = lower

	// The string is invalid if the last '1' is non-existent, it is the
	// first character of the string (no human-readable part) or one of the
	// last 6 characters of the string (since checksum cannot contain '1'),
	// or if the string is more than 90 characters in total.
	one := strings.LastIndexByte(bech, '1')
	if one < 1 || one+7 > len(bech) {
		return "", nil, fmt.Errorf("invalid index of 1")
	}

	// The human-readable part is everything before the last '1'.
	hrp := bech[:one]
	data := bech[one+1:]

	// Each character corresponds to the byte with value of the index in
	// 'charset'.
	decoded, err := toBytes(data)
	if err != nil {
		return "", nil, fmt.Errorf("failed converting data to bytes: "+
			"%v", err)
	}

	// We return the full decoded data body because we are not expecting a
	// checksum for bolt 12.
	return hrp, decoded[:], nil
}

// checkASCII checks that only ASCII characters between 33 and 126 are in a
// string.
func checkASCII(str string) error {
	for i := 0; i < len(str); i++ {
		if str[i] < 33 || str[i] > 126 {
			return fmt.Errorf("invalid character in "+
				"string: '%c'", str[i])
		}
	}

	return nil
}

// toBytes converts each character in the string 'chars' to the value of the
// index of the corresponding character in 'charset'.
func toBytes(chars string) ([]byte, error) {
	decoded := make([]byte, 0, len(chars))
	for i := 0; i < len(chars); i++ {
		index := strings.IndexByte(charset, chars[i])
		if index < 0 {
			return nil, fmt.Errorf("%w: %v", ErrNotInCharset,
				chars[i])
		}
		decoded = append(decoded, byte(index))
	}
	return decoded, nil
}

// stripOffer strips out any allowed "+" characters, validating that they are
// surrounded by bech32 characters. The offer string with the "+" characters
// removed is returned.
func stripOffer(offer string) (string, error) {
	parts := strings.Split(offer, "+")

	for i, part := range parts {
		// If any of our parts are empty, we either had consecutive or
		// starting / ending "+" in our string.
		if part == "" {
			return "", ErrIncorrectSplit
		}

		// We should allow whitespace following a "+" character. Trim
		// all whitespace so that we can check that except for spaces,
		// all other characters are bech32. We replace the entry in our
		// slice to strip out these paces.
		parts[i] = strings.TrimSpace(part)

		// Check that each split is in our charset.
		if err := checkASCII(parts[i]); err != nil {
			return "", fmt.Errorf("%w: part: %v", err, part)
		}
	}

	return strings.Join(parts, ""), nil
}

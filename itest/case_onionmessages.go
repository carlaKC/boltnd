package itest

import (
	"testing"

	"github.com/lightningnetwork/lnd/lntest"
)

// OnionMessageTestCase tests the exchange of onion messages.
func OnionMessageTestCase(t *testing.T, net *lntest.NetworkHarness) {
	offersTest := setupForBolt12(t, net)
	defer offersTest.cleanup()
}

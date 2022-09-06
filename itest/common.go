package itest

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/carlakc/boltnd/offersrpc"
	"github.com/stretchr/testify/require"
)

// assertBlindedPathEqual asserts that two blinded paths are equal.
func assertBlindedPathEqual(t *testing.T, expected,
	actual *offersrpc.BlindedPath) {

	require.Equal(t, expected.IntroductionNode, actual.IntroductionNode,
		"introduction")

	require.Equal(t, expected.BlindingPoint, actual.BlindingPoint,
		"blinding point")

	require.Equal(t, expected.IntroductionEncryptedData,
		actual.IntroductionEncryptedData, "introduction data")

	require.Equal(t, len(expected.Hops), len(actual.Hops), "hop count")

	for i, hop := range expected.Hops {
		require.Equal(t, hop.BlindedNodeId,
			actual.Hops[i].BlindedNodeId, "blinded node id", i)

		require.Equal(t, hop.EncryptedData,
			actual.Hops[i].EncryptedData, "encrypted data", i)
	}
}

// consumeOnionMessage sets up a closure that can be used to consume messages
// delivered from an onion message subscription client.
//
// This function calls Recv() in a goroutine so that tests will not block in
// the case where we expect to receive a message but one does not arrive. An
// alternative here would be to use a context with timeout on the top level
// subscription, but this would require callers to know how long the test will
// take to run (or overestimate it).
func consumeOnionMessage(wg *sync.WaitGroup,
	msgChan chan *offersrpc.SubscribeOnionPayloadResponse,
	errChan chan error) func(client offersrpc.Offers_SubscribeOnionPayloadClient) {

	return func(client offersrpc.Offers_SubscribeOnionPayloadClient) {
		wg.Add(1)
		go func() {
			defer wg.Done()

			msg, err := client.Recv()
			if err != nil {
				errChan <- err
				return
			}

			msgChan <- msg
		}()
	}
}

// readOnionMessage sets up a closure that will read responses from our onion
// message subscription channels or fail after a timeout.
func readOnionMessage(msgChan chan *offersrpc.SubscribeOnionPayloadResponse,
	errChan chan error) func() (*offersrpc.SubscribeOnionPayloadResponse,
	error) {

	return func() (*offersrpc.SubscribeOnionPayloadResponse, error) {
		select {
		// If we receive a message as expected, assert that it is of
		// the correct type.
		case msg := <-msgChan:
			return msg, nil

		// If we received an error, something went wrong.
		case err := <-errChan:
			return nil, err

		// In the case of a timeout, let our test exit. This will
		// cancel the receive goroutine (through context cancelation)
		// and wait for it to exit.
		case <-time.After(defaultTimeout):
			return nil, errors.New("message read timeout")
		}
	}
}

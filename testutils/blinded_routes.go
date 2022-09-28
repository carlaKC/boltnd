package testutils

import (
	"context"

	sphinx "github.com/lightningnetwork/lightning-onion"
	lndwire "github.com/lightningnetwork/lnd/lnwire"
	"github.com/stretchr/testify/mock"
)

// MockRouteGenerator creates a mock that substitutes for blinded route
// generation.
type MockRouteGenerator struct {
	*mock.Mock
}

// NewMockRouteGenerator produces a mock for blinded path generation.
func NewMockRouteGenerator() *MockRouteGenerator {
	return &MockRouteGenerator{
		Mock: &mock.Mock{},
	}
}

// ReplyPath mocks creation of a blinded route.
func (m *MockRouteGenerator) ReplyPath(ctx context.Context,
	features []lndwire.FeatureBit) (*sphinx.BlindedPath, error) {

	args := m.Mock.MethodCalled("BlindedRoute", ctx, features)
	return args.Get(0).(*sphinx.BlindedPath), args.Error(1)
}

// MockBlindedRoute primes our mock to return the error provided when
// send custom message is called with any CustomMessage.
func MockBlindedRoute(m *mock.Mock, features []lndwire.FeatureBit,
	path *sphinx.BlindedPath, err error) {

	m.On(
		"BlindedRoute", mock.Anything, features,
	).Once().Return(
		path, err,
	)
}

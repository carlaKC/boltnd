package offers

import (
	"testing"

	"github.com/carlakc/boltnd/testutils"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// offerCoordinatorTest contains the components required to run various tests
// concerning the offer coordinator.
type offerCoordinatorTest struct {
	t           *testing.T
	lnd         *testutils.MockLND
	mock        *mock.Mock
	coordinator *Coordinator
}

func newOfferCoordinatorTest(t *testing.T) *offerCoordinatorTest {
	testHelper := &offerCoordinatorTest{
		t:    t,
		lnd:  testutils.NewMockLnd(),
		mock: &mock.Mock{},
	}

	testHelper.coordinator = NewCoordinator(
		testHelper.lnd, testHelper.gracefulShutdown,
	)

	return testHelper
}

func (o *offerCoordinatorTest) start() {
	require.NoError(o.t, o.coordinator.Start(), "start coordinator")
}

func (o *offerCoordinatorTest) stop() {
	// Shutdown our coordinator.
	require.NoError(o.t, o.coordinator.Stop(), "stop coordinator")

	// Assert that we've made all the mocked calls we expect.
	o.mock.AssertExpectations(o.t)

	// Assert that our lnd mock has also had all the calls we expect.
	o.lnd.Mock.AssertExpectations(o.t)
}

// gracefulShutdown mocks a request for graceful shutdown due to the error
// provided.
func (o *offerCoordinatorTest) gracefulShutdown(err error) {
	o.mock.MethodCalled("gracefulShutdown", err)
}

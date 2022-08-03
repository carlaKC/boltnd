package offers

import (
	"testing"
	"time"

	"github.com/carlakc/boltnd/lnwire"
	"github.com/carlakc/boltnd/onionmsg"
	"github.com/carlakc/boltnd/testutils"
	"github.com/lightningnetwork/lnd/tlv"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var defaultTimeout = time.Second * 5

// offerCoordinatorTest contains the components required to run various tests
// concerning the offer coordinator.
type offerCoordinatorTest struct {
	t           *testing.T
	lnd         *testutils.MockLND
	mock        *mock.Mock
	coordinator *Coordinator

	// Embed our onion messenger interface so that the test embeds all
	// functionality by default.
	onionmsg.OnionMessenger
}

func newOfferCoordinatorTest(t *testing.T) *offerCoordinatorTest {
	testHelper := &offerCoordinatorTest{
		t:    t,
		lnd:  testutils.NewMockLnd(),
		mock: &mock.Mock{},
	}

	testHelper.coordinator = NewCoordinator(
		testHelper.lnd, testHelper, testHelper.gracefulShutdown,
	)

	return testHelper
}

func (o *offerCoordinatorTest) start() {
	// We always expect handler registration on start, so prime our mock
	// accordingly.
	mockRegisterHandler(
		o.mock, lnwire.InvoiceNamespaceType, nil,
	)

	require.NoError(o.t, o.coordinator.Start(), "start coordinator")
}

func (o *offerCoordinatorTest) stop() {
	// We always expect handler de-registration on stop, so prime our mock
	// accordingly.
	mockDeregisterHandler(o.mock, lnwire.InvoiceNamespaceType, nil)

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

// RegisterHandler mocks registering a handler.
func (o *offerCoordinatorTest) RegisterHandler(tlvType tlv.Type,
	handler onionmsg.OnionMessageHandler) error {

	args := o.mock.MethodCalled("RegisterHandler", tlvType, handler)
	return args.Error(0)
}

// mockRegisterHandler primes our mock to return the error provided when a call
// to register handler with tlvType (and any handler function) is called.
func mockRegisterHandler(m *mock.Mock, tlvType tlv.Type, err error) {
	m.On(
		"RegisterHandler", tlvType, mock.Anything,
	).Once().Return(
		err,
	)
}

// DeregisterHandler mocks deregistering a handler.
func (o *offerCoordinatorTest) DeregisterHandler(tlvType tlv.Type) error {
	args := o.mock.MethodCalled("DeregisterHandler", tlvType)
	return args.Error(0)
}

// mockDeregisterHandler primes our mock to return the error provided when a
// call to dereigster handler with tlvType is made.
func mockDeregisterHandler(m *mock.Mock, tlvType tlv.Type, err error) {
	m.On(
		"DeregisterHandler", tlvType,
	).Once().Return(
		err,
	)
}

package itest

import (
	"fmt"
	"testing"

	"github.com/go-errors/errors"
	"github.com/lightningnetwork/lnd/lntest"
)

type testCase struct {
	name string
	test func(*testing.T, *harnessTest)
}

// harnessTest wraps a regular testing.T providing enhanced error detection
// and propagation. All error will be augmented with a full stack-trace in
// order to aid in debugging. Additionally, any panics caused by active
// test cases will also be handled and represented as fatals.
type harnessTest struct {
	t *testing.T

	// lndHarness is a reference to the current network harness.
	lndHarness *lntest.NetworkHarness

	// bolt12
	bolt12 *bolt12TestSetup

	// testCase is populated during test execution and represents the
	// current test case.
	testCase *testCase
}

func newHarnessTest(t *testing.T, lnd *lntest.NetworkHarness) *harnessTest {
	return &harnessTest{
		t:          t,
		lndHarness: lnd,
		bolt12:     setupForBolt12(t, lnd),
		testCase:   nil,
	}
}

func (h *harnessTest) cleanUp() {
	if h.bolt12 == nil {
		return
	}

	h.bolt12.cleanup()
}

// Skipf calls the underlying testing.T's Skip method, causing the current test
// to be skipped.
func (h *harnessTest) Skipf(format string, args ...interface{}) {
	h.t.Skipf(format, args...)
}

// Fatalf causes the current active test case to fail with a fatal error. All
// integration tests should mark test failures solely with this method due to
// the error stack traces it produces.
func (h *harnessTest) Fatalf(format string, a ...interface{}) {
	if h.lndHarness != nil {
		h.lndHarness.SaveProfilesPages(h.t)
	}

	stacktrace := errors.Wrap(fmt.Sprintf(format, a...), 1).ErrorStack()
	if h.testCase != nil {
		h.t.Fatalf("Failed: (%v): exited with error: \n"+
			"%v", h.testCase.name, stacktrace)
	} else {
		h.t.Fatalf("Error outside of test: %v", stacktrace)
	}
}

// RunTestCase executes a harness test case. Any errors or panics will be
// represented as fatal.
func (h *harnessTest) RunTestCase(testCase *testCase) {
	h.testCase = testCase
	defer func() {
		h.testCase = nil
	}()

	defer func() {
		if err := recover(); err != nil {
			description := errors.Wrap(err, 2).ErrorStack()
			h.t.Fatalf("Failed: (%v) panicked with: \n%v",
				h.testCase.name, description)
		}
	}()

	testCase.test(h.t, h)
}

func (h *harnessTest) Logf(format string, args ...interface{}) {
	h.t.Logf(format, args...)
}

func (h *harnessTest) Log(args ...interface{}) {
	h.t.Log(args...)
}

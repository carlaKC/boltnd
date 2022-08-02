package offers

import "testing"

// TestCoordinatorLifecycle tests starting and shutdown of the coordinator.
func TestCoordinatorLifecycle(t *testing.T) {
	// Creating a test case and starting/stopping it will assert that we
	// shutdown on stop with no errors.
	coordinatorTest := newOfferCoordinatorTest(t)
	coordinatorTest.start()
	coordinatorTest.stop()
}

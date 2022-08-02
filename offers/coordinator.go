package offers

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
)

var (
	// ErrShuttingDown is returned when the coordinator exits.
	ErrShuttingDown = errors.New("coordinator shutting down")
)

// Coordinator manages the exchange of offer-related messages and payment of
// invoices.
type Coordinator struct {
	started int32 // to be used atomically
	stopped int32 // to be used atomically

	// requestShutdown is called when the messenger experiences an error to
	// signal to calling code that it should gracefully exit.
	requestShutdown func(err error)

	wg   sync.WaitGroup
	quit chan struct{}
}

// NewCoordinator creates a new offer coordinator.
func NewCoordinator(requestShutdown func(err error)) *Coordinator {
	return &Coordinator{
		requestShutdown: requestShutdown,
		quit:            make(chan struct{}),
	}
}

// Start runs the coordinator.
func (c *Coordinator) Start() error {
	if !atomic.CompareAndSwapInt32(&c.started, 0, 1) {
		return errors.New("coordinator already started")
	}

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()

		err := c.handleOffers()
		if err != nil && err != ErrShuttingDown {
			c.requestShutdown(err)
		}
	}()

	return nil
}

// Stop shuts down the coordinator.
func (c *Coordinator) Stop() error {
	if !atomic.CompareAndSwapInt32(&c.stopped, 0, 1) {
		return fmt.Errorf("coordinator already stopped")
	}

	close(c.quit)
	c.wg.Wait()

	return nil
}

// handleOffers is the main goroutine that handles offer exchanges.
func (c *Coordinator) handleOffers() error {
	for {
		select {
		case <-c.quit:
			return ErrShuttingDown
		}
	}
}

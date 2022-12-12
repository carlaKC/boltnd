package itest

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/integration/rpctest"
	"github.com/lightningnetwork/lnd/lntest"
	"github.com/stretchr/testify/require"
)

var (
	harnessNetParams = &chaincfg.RegressionNetParams
)

func TestBoltnd(t *testing.T) {
	// If no tests are registered, then we can exit early.
	if len(testCases) == 0 {
		t.Skip("integration tests not selected with flag 'rpctest'")
	}

	// Parse testing flags that influence our test execution.
	logDir := lntest.GetLogDir()
	require.NoError(t, os.MkdirAll(logDir, 0700))

	// Before we start any node, we need to make sure that any btcd node
	// that is started through the RPC harness uses a unique port as well to
	// avoid any port collisions.
	rpctest.ListenAddressGenerator = lntest.GenerateBtcdListenerAddresses

	// Declare the network harness here to gain access to its
	// 'OnTxAccepted' call back.
	var lndHarness *lntest.NetworkHarness

	// Create an instance of the btcd's rpctest.Harness that will act as
	// the miner for all tests. This will be used to fund the wallets of
	// the nodes within the test network and to drive blockchain related
	// events within the network. Revert the default setting of accepting
	// non-standard transactions on simnet to reject them. Transactions on
	// the lightning network should always be standard to get better
	// guarantees of getting included in to blocks.
	//
	// We will also connect it to our chain backend.
	miner, err := lntest.NewMiner()
	require.NoError(t, err, "failed to create new miner")
	defer func() {
		require.NoError(t, miner.Stop(), "failed to stop miner")
	}()

	// Start a chain backend.
	chainBackend, cleanUp, err := lntest.NewBackend(
		miner.P2PAddress(), harnessNetParams,
	)
	require.NoError(t, err, "new backend")
	defer func() {
		require.NoError(t, cleanUp(), "cleanup")
	}()

	// Before we start anything, we want to overwrite some of the connection
	// settings to make the tests more robust. We might need to restart the
	// miner while there are already blocks present, which will take a bit
	// longer than the 1 second the default settings amount to. Doubling
	// both values will give us retries up to 4 seconds.
	miner.MaxConnRetries = rpctest.DefaultMaxConnectionRetries * 2
	miner.ConnectionRetryTimeout = rpctest.DefaultConnectionRetryTimeout * 2

	// Set up miner and connect chain backend to it.
	require.NoError(t, miner.SetUp(true, 50))
	require.NoError(t, miner.Client.NotifyNewTransactions(false))
	require.NoError(t, chainBackend.ConnectMiner(), "connect miner")

	// Now we can set up our test harness (LND instance), with the chain
	// backend we just created.
	lndHarness, err = lntest.NewNetworkHarness(
		miner, chainBackend, "./lnd-itest", lntest.BackendBbolt,
	)
	if err != nil {
		t.Fatalf("unable to create lightning network harness: %v", err)
	}
	defer lndHarness.Stop()

	// Spawn a new goroutine to watch for any fatal errors that any of the
	// running lnd processes encounter. If an error occurs, then the test
	// case should naturally as a result and we log the server error here to
	// help debug.
	go func() {
		for {
			select {
			case err, more := <-lndHarness.ProcessErrors():
				if !more {
					return
				}
				t.Logf("lnd finished with error (stderr):\n%v",
					err)
			}
		}
	}()

	// Next mine enough blocks in order for segwit and the CSV package
	// soft-fork to activate on SimNet.
	numBlocks := harnessNetParams.MinerConfirmationWindow * 2
	if _, err := miner.Client.Generate(numBlocks); err != nil {
		t.Fatalf("unable to generate blocks: %v", err)
	}

	// With the btcd harness created, we can now complete the
	// initialization of the network. args - list of lnd arguments,
	// example: "--debuglevel=debug"
	// TODO(roasbeef): create master balanced channel with all the monies?
	aliceBobArgs := []string{
		"--default-remote-max-htlcs=483",
		"--dust-threshold=5000000",
	}

	// Run the subset of the test cases selected in this tranche.
	for _, testCase := range testCases {
		testCase := testCase
		name := fmt.Sprintf("%s/%s", chainBackend.Name(), testCase.name)

		success := t.Run(name, func(t1 *testing.T) {
			cleanTestCaseName := strings.ReplaceAll(
				testCase.name, " ", "_",
			)

			err = lndHarness.SetUp(
				t1, cleanTestCaseName, aliceBobArgs,
			)
			require.NoError(t1,
				err, "unable to set up test lightning network",
			)

			ht := newHarnessTest(t1, lndHarness)

			defer func() {
				// Clean up offers stub + lnd.
				ht.cleanUp()
				require.NoError(t1, lndHarness.TearDown())
			}()

			lndHarness.EnsureConnected(
				t1, lndHarness.Alice, lndHarness.Bob,
			)

			logLine := fmt.Sprintf(
				"STARTING ============ %v ============\n",
				testCase.name,
			)

			lndHarness.Alice.AddToLogf(logLine)
			lndHarness.Bob.AddToLogf(logLine)

			// Start every test with the default static fee estimate.
			lndHarness.SetFeeEstimate(12500)

			// Create a separate harness test for the testcase to
			// avoid overwriting the external harness test that is
			// tied to the parent test.
			ht.RunTestCase(testCase)
		})

		// Stop at the first failure. Mimic behavior of original test
		// framework.
		if !success {
			// Log failure time to help relate the lnd logs to the
			// failure.
			t.Logf("Failure time: %v", time.Now().Format(
				"2006-01-02 15:04:05.000",
			))
			break
		}
	}

}

package cmd

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/luxfi/genesis/pkg/application"
	"github.com/luxfi/genesis/pkg/replay"
	"github.com/luxfi/go-ethereum/core/types"
	"github.com/luxfi/go-ethereum/ethclient"
	"github.com/spf13/cobra"
)

// NewReplayCmd creates the replay command, now incorporating pre-flight checks
// from the `manual-replay.sh` script.
func NewReplayCmd(app *application.Genesis) *cobra.Command {
	var opts replay.Options

	cmd := &cobra.Command{
		Use:   "replay [source-db]",
		Short: "Replay blockchain blocks with pre-flight checks",
		Long:  "Checks node status, then reads finalized blocks from a database and replays them into the C-Chain.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// --- Pre-flight checks from manual-replay.sh ---
			fmt.Println("🔄 Manual Blockchain Replay Script")
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

			fmt.Println("Checking if node is running...")
			client, err := ethclient.Dial(opts.RPC)
			if err != nil {
				return fmt.Errorf("❌ Error: Node is not running or RPC is not accessible at %s. Please start the node first", opts.RPC)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			blockNumber, err := client.BlockNumber(ctx)
			if err != nil {
				return fmt.Errorf("❌ Error: Could not get block number from RPC: %w", err)
			}

			fmt.Printf("✓ Node is running. Current block: %d\n", blockNumber)

			if blockNumber > 0 {
				fmt.Println("⚠️  Chain already has blocks. Manual replay not needed.")
				return nil
			}
			// --- End of pre-flight checks ---

			fmt.Println("📊 Starting manual blockchain replay...")
			fmt.Println("This will replay blocks from the imported database into the running chain.")

			replayer := replay.New(app)
			if err := replayer.ReplayBlocks(args[0], opts); err != nil {
				return err
			}

			// Final status check
			fmt.Println("✅ Replay complete!")
			finalBlockNumber, err := client.BlockNumber(ctx)
			if err != nil {
				return fmt.Errorf("could not get final block number: %w", err)
			}
			fmt.Printf("Final block height: %d\n", finalBlockNumber)

			return nil
		},
	}

	cmd.Flags().StringVar(&opts.RPC, "rpc", "http://localhost:9630/ext/bc/C/rpc", "RPC endpoint for pre-flight checks and replay")
	cmd.Flags().Uint64Var(&opts.Start, "start", 0, "Start block (0 = genesis)")
	cmd.Flags().Uint64Var(&opts.End, "end", 0, "End block (0 = all)")
	cmd.Flags().BoolVar(&opts.DirectDB, "direct-db", false, "Write directly to database instead of RPC")
	cmd.Flags().StringVar(&opts.Output, "output", "", "Output database path (for direct-db mode)")

	// Add subcommands
	cmd.AddCommand(newReplayBlocksCmd(app))
	cmd.AddCommand(newReplayWithLoggingCmd(app))
	cmd.AddCommand(newTestReplayCmd(app))

	return cmd
}

// newReplayBlocksCmd creates the command for replaying blocks from a database to a running node.
// This replaces the functionality of the `replay-blocks.sh` script.
func newReplayBlocksCmd(app *application.Genesis) *cobra.Command {
	var opts replay.Options

	cmd := &cobra.Command{
		Use:   "blocks [source-db]",
		Short: "Replay blocks from a database to a running node",
		Long:  "Reads blocks from a source database and replays them to a running node via RPC.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("🚀 Starting block replay...")

			replayer := replay.New(app)
			if err := replayer.ReplayBlocks(args[0], opts); err != nil {
				return err
			}

			fmt.Println("✅ Replay complete!")
			return nil
		},
	}

	cmd.Flags().StringVar(&opts.RPC, "rpc", "http://localhost:9630/ext/bc/C/rpc", "RPC endpoint for replaying blocks")
	cmd.Flags().Uint64Var(&opts.Start, "start", 1, "Start block")
	cmd.Flags().Uint64Var(&opts.End, "end", 100, "End block")

	return cmd
}

// newReplayWithLoggingCmd creates the command for replaying blocks with detailed logging and monitoring.
// This replaces the functionality of the `run-replay-with-logging.sh` script.
func newReplayWithLoggingCmd(app *application.Genesis) *cobra.Command {
	var (
		opts      replay.Options
		logFile   string
		batchSize uint64
	)

	cmd := &cobra.Command{
		Use:   "with-logging [source-db]",
		Short: "Replay blocks with detailed logging and monitoring",
		Long:  "Replays blocks from a source database with detailed logging and progress monitoring.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Set up logger
			if logFile == "" {
				logFile = fmt.Sprintf("/tmp/block-replay-%s.log", time.Now().Format("20060102-150405"))
			}
			f, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
			if err != nil {
				return fmt.Errorf("error opening log file: %v", err)
			}
			defer f.Close()
			log.SetOutput(f)

			log.Println("Block Replay Script")
			log.Println("==================")
			log.Printf("Database: %s", args[0])
			log.Printf("Log file: %s", logFile)

			// Check if node is running
			client, err := ethclient.Dial(opts.RPC)
			if err != nil {
				return fmt.Errorf("❌ Error: Node is not running or RPC is not accessible at %s. Please start the node first", opts.RPC)
			}

			// Get current block height
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			startBlock, err := client.BlockNumber(ctx)
			if err != nil {
				return fmt.Errorf("❌ Error: Could not get block number from RPC: %w", err)
			}
			log.Printf("Current block height: %d", startBlock)

			// Start monitoring
			monitorCtx, stopMonitor := context.WithCancel(context.Background())
			go monitorReplay(monitorCtx, client)

			// Handle shutdown signals
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
			go func() {
				<-sigCh
				log.Println("Shutting down...")
				stopMonitor()
			}()

			// Run replay in batches
			replayer := replay.New(app)
			for i := startBlock; i < opts.End; i += batchSize {
				batchEnd := i + batchSize - 1
				if batchEnd > opts.End {
					batchEnd = opts.End
				}
				log.Printf("Replaying batch: blocks %d to %d", i, batchEnd)
				batchOpts := opts
				batchOpts.Start = i
				batchOpts.End = batchEnd
				if err := replayer.ReplayBlocks(args[0], batchOpts); err != nil {
					log.Printf("ERROR: Replay failed for batch %d-%d: %v", i, batchEnd, err)
				}
				time.Sleep(500 * time.Millisecond)
			}

			// Stop monitoring and print final status
			stopMonitor()
			log.Println("Replay complete. Checking final status...")
			finalHeight, err := client.BlockNumber(ctx)
			if err != nil {
				log.Printf("Could not get final block number: %v", err)
			} else {
				log.Printf("Final block height: %d", finalHeight)
				if finalHeight > startBlock {
					log.Printf("SUCCESS: Replayed %d blocks", finalHeight-startBlock)
				} else {
					log.Println("WARNING: No new blocks were replayed")
				}
			}
			log.Printf("Full log available at: %s", logFile)

			return nil
		},
	}

	cmd.Flags().StringVar(&opts.RPC, "rpc", "http://localhost:9630/ext/bc/C/rpc", "RPC endpoint for replaying blocks")
	cmd.Flags().Uint64Var(&opts.End, "end", 1082781, "End block")
	cmd.Flags().StringVar(&logFile, "log-file", "", "Path to log file (default: /tmp/block-replay-TIMESTAMP.log)")
	cmd.Flags().Uint64Var(&batchSize, "batch-size", 100, "Number of blocks to replay in each batch")

	return cmd
}

func monitorReplay(ctx context.Context, client *ethclient.Client) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			height, err := client.BlockNumber(ctx)
			if err != nil {
				fmt.Printf("\rCurrent Height: error getting height")
			} else {
				fmt.Printf("\rCurrent Height: %d    ", height)
			}
		}
	}
}

// newTestReplayCmd creates the command for testing the replay functionality.
// This replaces the functionality of the `test-replay.sh` script.
func newTestReplayCmd(app *application.Genesis) *cobra.Command {
	var (
		rpcURL  string
		dbPath  string
		monitor bool
	)

	cmd := &cobra.Command{
		Use:   "test",
		Short: "Test the replay functionality",
		Long:  "Tests the replay functionality by checking the node status and inspecting replayed blocks.",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Println("Testing block replay functionality")

			client, err := ethclient.Dial(rpcURL)
			if err != nil {
				return fmt.Errorf("luxd is not running or RPC is not accessible at %s", rpcURL)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			height, err := client.BlockNumber(ctx)
			if err != nil {
				return fmt.Errorf("could not get block number from RPC: %w", err)
			}
			log.Printf("Current blockchain height: %d", height)

			testBlocks := []uint64{0, 1, 10, 100, 1000, 10000, 100000}
			log.Println("Checking replayed blocks...")
			for _, blockNum := range testBlocks {
				if blockNum > height {
					break
				}
				block, err := client.BlockByNumber(ctx, new(big.Int).SetUint64(blockNum))
				if err != nil {
					log.Printf("Block %d not found", blockNum)
					continue
				}
				log.Printf("Block %d: hash=%s... timestamp=%d txs=%d",
					blockNum, block.Hash().Hex()[:16], block.Time(), len(block.Transactions()))
			}

			log.Println("\n=== Replay Status Summary ===")
			log.Printf("Current height: %d", height)
			if height > 1000 {
				log.Println("Status: Replay appears to be working")
			} else {
				log.Println("Status: Replay in progress or not started")
			}

			if monitor {
				log.Println("\nMonitoring replay progress (press Ctrl+C to stop)...")
				monitorReplay(context.Background(), client)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&rpcURL, "rpc", "http://localhost:9630/ext/bc/C/rpc", "RPC endpoint for testing")
	cmd.Flags().StringVar(&dbPath, "db-path", "state/chaindata/lux-mainnet-96369/db", "Path to the source database")
	cmd.Flags().BoolVar(&monitor, "monitor", false, "Monitor replay progress")

	return cmd
}

// NewSubnetBlockReplayCmd creates the subnet-block-replay command (alias for replay with direct-db)
func NewSubnetBlockReplayCmd(app *application.Genesis) *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "subnet-block-replay [source-db]",
		Short: "Replay subnet blocks directly to database",
		Long:  "Replay blocks from SubnetEVM database directly to output database (equivalent to replay --direct-db)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := replay.Options{
				DirectDB: true,
				Output:   output,
			}
			replayer := replay.New(app)
			return replayer.ReplayBlocks(args[0], opts)
		},
	}

	cmd.Flags().StringVar(&output, "output", "", "Output database path (required)")
	_ = cmd.MarkFlagRequired("output")

	return cmd
}

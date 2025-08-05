package cmd

import (
	"fmt"
	"time"

	"github.com/luxfi/genesis/pkg/application"
	"github.com/luxfi/node/consensus/sampling"
	"github.com/spf13/cobra"
)

// NewConsensusCmd creates the consensus command
func NewConsensusCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "consensus",
		Short: "Consensus parameter management",
		Long:  "Commands for managing and validating consensus parameters",
	}

	cmd.AddCommand(newValidateCmd(app))
	cmd.AddCommand(newShowCmd(app))

	return cmd
}

func newValidateCmd(app *application.Genesis) *cobra.Command {
	var (
		k                  int
		alphaPreference    int
		alphaConfidence    int
		beta               int
		concurrentRepolls  int
		optimalProcessing  int
		maxOutstandingItems int
		maxItemProcessingTime time.Duration
	)

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate consensus parameters",
		Long:  "Validate that consensus parameters are valid for the given configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			params := sampling.Parameters{
				K:                     k,
				AlphaPreference:       alphaPreference,
				AlphaConfidence:       alphaConfidence,
				Beta:                  beta,
				ConcurrentRepolls:     concurrentRepolls,
				OptimalProcessing:     optimalProcessing,
				MaxOutstandingItems:   maxOutstandingItems,
				MaxItemProcessingTime: maxItemProcessingTime,
			}

			// Validate parameters
			if err := params.Verify(); err != nil {
				return fmt.Errorf("invalid consensus parameters: %w", err)
			}

			fmt.Println("✅ Consensus parameters are valid")
			fmt.Printf("\nConfiguration:\n")
			fmt.Printf("  K (sample size): %d\n", k)
			fmt.Printf("  Alpha Preference: %d (> %d)\n", alphaPreference, k/2)
			fmt.Printf("  Alpha Confidence: %d\n", alphaConfidence)
			fmt.Printf("  Beta (finalization): %d\n", beta)
			fmt.Printf("  Concurrent Repolls: %d\n", concurrentRepolls)
			fmt.Printf("  Min Connected: %.1f%%\n", params.MinPercentConnectedHealthy()*100)

			if k == 1 {
				fmt.Println("\n🔧 Single Node Configuration:")
				fmt.Println("  - BFT consensus with 1 validator")
				fmt.Println("  - Requires 100% connectivity (self-connected)")
				fmt.Println("  - BLS aggregate signature of 1")
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&k, "k", 20, "Sample size")
	cmd.Flags().IntVar(&alphaPreference, "alpha-preference", 15, "Vote threshold to change preference")
	cmd.Flags().IntVar(&alphaConfidence, "alpha-confidence", 15, "Vote threshold to increase confidence")
	cmd.Flags().IntVar(&beta, "beta", 20, "Consecutive successful queries for finalization")
	cmd.Flags().IntVar(&concurrentRepolls, "concurrent-repolls", 4, "Concurrent outstanding polls")
	cmd.Flags().IntVar(&optimalProcessing, "optimal-processing", 10, "Optimal processing limit")
	cmd.Flags().IntVar(&maxOutstandingItems, "max-outstanding-items", 256, "Max outstanding items")
	cmd.Flags().DurationVar(&maxItemProcessingTime, "max-item-processing-time", 30*time.Second, "Max item processing time")

	return cmd
}

func newShowCmd(app *application.Genesis) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show consensus configurations",
		Long:  "Display various consensus parameter configurations",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("=== Consensus Parameter Configurations ===")

			// Default configuration
			fmt.Println("📊 Default Configuration (k=20):")
			fmt.Println("  - K: 20")
			fmt.Println("  - Alpha Preference: 15")
			fmt.Println("  - Alpha Confidence: 15")
			fmt.Println("  - Beta: 20")
			fmt.Println("  - Concurrent Repolls: 4")

			// Single node configuration
			fmt.Println("\n🔧 Single Node Configuration (k=1):")
			fmt.Println("  - K: 1")
			fmt.Println("  - Alpha Preference: 1")
			fmt.Println("  - Alpha Confidence: 1")
			fmt.Println("  - Beta: 1 (quick) or 20 (production)")
			fmt.Println("  - Concurrent Repolls: 1")

			// Test configuration
			fmt.Println("\n🧪 Test Configuration (k=5):")
			fmt.Println("  - K: 5")
			fmt.Println("  - Alpha Preference: 3")
			fmt.Println("  - Alpha Confidence: 4")
			fmt.Println("  - Beta: 5")
			fmt.Println("  - Concurrent Repolls: 2")

			return nil
		},
	}

	return cmd
}
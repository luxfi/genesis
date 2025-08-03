# 6. Development & Contribution

This guide is for developers who want to contribute to the Lux Genesis project, run its test suite, or use its packages as a library in their own Go applications.

## Running the Test Suite

The project includes a comprehensive test suite using the [Ginkgo](https://onsi.github.io/ginkgo/) testing framework. The tests cover everything from individual package functions to full network integration tests.

### Prerequisites

-   Go 1.24.5+
-   `make`
-   All test dependencies, which can be installed via the Makefile.

### Install Test Dependencies

This command will download the `ginkgo` CLI and other Go dependencies required for the tests.

```bash
make install-test-deps
```

### Running Tests

The `Makefile` provides several targets for running different subsets of the tests.

**1. Run All Tests (Recommended)**

This is the easiest way to run the entire suite, including unit and integration tests.

```bash
make test-all
```

**2. Run Unit Tests Only**

This runs the tests for individual packages, which is fast and doesn't require network components.

```bash
make test-unit
```

**3. Run Integration Tests Only**

This runs the tests that involve spinning up nodes, deploying networks, and importing data. These tests are slower and more resource-intensive.

```bash
make test-integration
```

### Running Specific Tests

You can use the `ginkgo` CLI directly to run specific tests by focusing on their descriptions.

```bash
# Test only the database operations
ginkgo -v --focus="Database Operations" tests/

# Test only the 5-node primary network setup
ginkgo -v --focus="5-Node Primary Network" tests/integration/
```

### Debugging Failed Tests

-   **Check Logs**: Node logs can be found in `~/.luxd/logs/` and CLI logs in `~/.avalanche-cli/logs/`.
-   **Clean Test Data**: If tests fail due to state conflicts, you can clean up old test networks with `rm -rf ~/.avalanche-cli/networks/test-*`.

---

## Using as a Go Library

The `genesis` tool is built on a set of reusable Go packages located in the `pkg/` directory. You can import these into your own projects to programmatically build genesis files or interact with Lux network configurations.

### Example: Programmatically Building a Genesis File

This example demonstrates how to use the `genesis` package to create a new builder, add a token allocation, and build a genesis configuration.

```go
package main

import (
	"fmt"
	"math/big"

	"github.com/luxfi/genesis/pkg/genesis"
	"github.com/luxfi/genesis/pkg/genesis/allocation"
)

func main() {
	// Create a new genesis builder for the 'local' network configuration
	builder, err := genesis.NewBuilder("local")
	if err != nil {
		panic(err)
	}

	// Create a new token allocation
	// Address: 0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC
	// Amount: 1,000,000,000,000,000,000 (1 LUX, since C-Chain has 18 decimals)
	addr := "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC"
	amount := new(big.Int)
	amount.SetString("1000000000000000000", 10)

	alloc := allocation.Allocation{
		Address: addr,
		Amount:  amount,
	}

	// Add the allocation to the builder
	if err := builder.AddAllocation(alloc); err != nil {
		panic(err)
	}

	// Build the final genesis object
	g, err := builder.Build()
	if err != nil {
		panic(err)
	}

	// Save the genesis object to a file
	if err := builder.SaveToFile(g, "my_custom_genesis.json"); err != nil {
		panic(err)
	}

	fmt.Println("Successfully created my_custom_genesis.json")
}
```

### Key Packages

-   `pkg/genesis`: The main package for the genesis builder.
-   `pkg/genesis/allocation`: Handles token allocation management.
-   `pkg/genesis/config`: Manages network configurations.
-   `pkg/genesis/validator`: Provides tools for validator key generation and management.
-   `pkg/migration`: Contains the core logic for data migration.

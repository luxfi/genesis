# Genesis Configuration and Management Makefile
# For Lux Network, Zoo L2, and Quantum Chains

SHELL := /bin/bash
.PHONY: all clean build install test clone-state update-state

# Directories
STATE_DIR := state
CHAINDATA_DIR := $(STATE_DIR)/chaindata
BUILD_DIR := bin
CONFIG_DIR := configs

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOMOD := $(GOCMD) mod

# Binary name
BINARY_NAME := genesis

# State repository (can be overridden)
# Use HTTPS by default for CI compatibility, can override with SSH for local dev
STATE_REPO ?= https://github.com/luxfi/state.git
STATE_LOCAL ?= ../state

all: build

# Build the genesis CLI tool with all database support
build:
	@echo "Building genesis CLI with all database support..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -tags "pebbledb rocksdb" -o $(BUILD_DIR)/$(BINARY_NAME) .

install: build
	@echo "Installing genesis CLI..."
	@cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/

# Clone state repository for historic chaindata (on-demand)
clone-state:
	@echo "Cloning state repository for historic chaindata..."
	@if [ -d "$(STATE_DIR)" ]; then \
		echo "State directory already exists. Use 'make update-state' to update."; \
	else \
		if [ -d "$(STATE_LOCAL)" ]; then \
			echo "Copying from local state at $(STATE_LOCAL)..."; \
			mkdir -p $(STATE_DIR); \
			cp -r $(STATE_LOCAL)/chaindata $(STATE_DIR)/ 2>/dev/null || echo "Warning: chaindata not found"; \
			cp -r $(STATE_LOCAL)/configs $(STATE_DIR)/ 2>/dev/null || echo "Warning: configs not found"; \
		else \
			echo "Cloning from remote $(STATE_REPO)..."; \
			git clone --depth 1 --sparse $(STATE_REPO) $(STATE_DIR).tmp; \
			cd $(STATE_DIR).tmp && git sparse-checkout set chaindata configs; \
			mv $(STATE_DIR).tmp/* $(STATE_DIR)/ 2>/dev/null || true; \
			mv $(STATE_DIR).tmp/.git $(STATE_DIR)/ 2>/dev/null || true; \
			rm -rf $(STATE_DIR).tmp; \
		fi; \
		echo "State cloned successfully."; \
	fi

# Update existing state data
update-state:
	@if [ -d "$(STATE_DIR)" ]; then \
		echo "Updating state data from $(STATE_REPO)..."; \
		rsync -av --delete $(STATE_REPO)/chaindata/ $(STATE_DIR)/chaindata/; \
		rsync -av --delete $(STATE_REPO)/configs/ $(STATE_DIR)/configs/; \
		echo "State updated successfully."; \
	else \
		echo "State directory not found. Run 'make clone-state' first."; \
		exit 1; \
	fi

# Clean build artifacts (but not state data)
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@$(GOCMD) clean

# Deep clean including state data
deep-clean: clean
	@echo "Removing cloned state data..."
	@rm -rf $(STATE_DIR)

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Ginkgo test targets
install-test-deps:
	@echo "Installing test dependencies..."
	go install github.com/onsi/ginkgo/v2/ginkgo@latest

test-ginkgo: install-test-deps
	@echo "Running Ginkgo tests..."
	ginkgo -r --race --cover --trace --progress test/

test-unit: install-test-deps
	@echo "Running unit tests..."
	ginkgo -r --race --cover --trace --progress --label-filter="!integration" test/

test-integration: install-test-deps
	@echo "Running integration tests..."
	ginkgo -r --race --cover --trace --progress --label-filter="integration" test/

test-migration: install-test-deps
	@echo "Running migration tests..."
	ginkgo --race --cover --trace --progress test/migration/

test-validators: install-test-deps
	@echo "Running validator tests..."
	ginkgo --race --cover --trace --progress test/validators/

test-genesis: install-test-deps
	@echo "Running genesis tests..."
	ginkgo --race --cover --trace --progress test/genesis/

test-all: test test-ginkgo

# Generate genesis configurations
gen-l1:
	@echo "Generating L1 genesis configuration..."
	$(BUILD_DIR)/$(BINARY_NAME) generate --type l1 --network mainnet

gen-l2:
	@echo "Generating L2 genesis configuration..."
	$(BUILD_DIR)/$(BINARY_NAME) generate --type l2 --network zoo-mainnet --base-chain lux

gen-l3:
	@echo "Generating L3 genesis configuration..."
	$(BUILD_DIR)/$(BINARY_NAME) generate --type l3 --network custom --base-chain zoo

# Quick commands for all 8 chains
lux-mainnet:
	$(BUILD_DIR)/$(BINARY_NAME) generate --network lux-mainnet

lux-testnet:
	$(BUILD_DIR)/$(BINARY_NAME) generate --network lux-testnet

lux-local:
	$(BUILD_DIR)/$(BINARY_NAME) generate --network lux-local

zoo-mainnet:
	$(BUILD_DIR)/$(BINARY_NAME) generate --network zoo-mainnet

zoo-testnet:
	$(BUILD_DIR)/$(BINARY_NAME) generate --network zoo-testnet

spc-mainnet:
	$(BUILD_DIR)/$(BINARY_NAME) generate --network spc-mainnet

spc-testnet:
	$(BUILD_DIR)/$(BINARY_NAME) generate --network spc-testnet

hanzo-mainnet:
	$(BUILD_DIR)/$(BINARY_NAME) generate --network hanzo-mainnet

hanzo-testnet:
	$(BUILD_DIR)/$(BINARY_NAME) generate --network hanzo-testnet

# Generate all chains
gen-all: lux-mainnet lux-testnet zoo-mainnet zoo-testnet spc-mainnet spc-testnet hanzo-mainnet hanzo-testnet

# Launch commands
launch: pipeline-lux node
	@echo "✅ Full pipeline complete! Node is running."

node: check-processed-data
	@echo "🚀 Starting Lux Network node..."
	@echo ""
	@echo "Network: LUX Mainnet (96369)"
	@echo "Blocks: 1,082,781"  
	@echo "Data: state/processed/lux-mainnet-96369/C/db"
	@echo "RPC: http://localhost:9630/ext/bc/C/rpc"
	@echo ""
	@$(BUILD_DIR)/$(BINARY_NAME) launch migrated --data-dir state/processed/lux-mainnet-96369/C/db --network-id 96369

pipeline-lux: build extract-blockchain migrate-blockchain
	@echo "✅ Pipeline complete for LUX mainnet"

check-processed-data:
	@if [ ! -d "state/processed/lux-mainnet-96369/C/db" ]; then \
		echo "❌ Error: Processed blockchain data not found!"; \
		echo ""; \
		echo "Run this command first:"; \
		echo "  make pipeline-lux"; \
		echo ""; \
		exit 1; \
	fi

extract-blockchain:
	@if [ ! -d "extracted-blockchain/pebbledb" ]; then \
		echo "📦 Extracting blockchain data from state repo..."; \
		mkdir -p extracted-blockchain; \
		$(BUILD_DIR)/$(BINARY_NAME) extract-blockchain \
			state/chaindata/lux-mainnet-96369/db/pebbledb \
			extracted-blockchain/pebbledb; \
	else \
		echo "✓ Blockchain data already extracted"; \
	fi

migrate-blockchain: extract-blockchain
	@if [ ! -d "state/processed/lux-mainnet-96369/C/db" ]; then \
		echo "🔄 Migrating blockchain data to C-Chain format..."; \
		mkdir -p state/processed/lux-mainnet-96369/C; \
		$(BUILD_DIR)/$(BINARY_NAME) subnet-block-replay \
			extracted-blockchain/pebbledb \
			--output state/processed/lux-mainnet-96369/C/db \
			--direct-db; \
	else \
		echo "✓ Blockchain data already migrated"; \
	fi

# Legacy launch targets (deprecated - use 'make launch' instead)
launch-mainnet: launch
launch-l1: launch
launch-l2: pipeline-zoo node
launch-l3: pipeline-spc node

# Quantum chain specific targets
quantum-genesis:
	@echo "Generating quantum chain genesis..."
	$(BUILD_DIR)/$(BINARY_NAME) generate --network quantum-mainnet

quantum-launch:
	@echo "Launching quantum chain..."
	$(BUILD_DIR)/$(BINARY_NAME) launch --type quantum --config $(CONFIG_DIR)/quantum-genesis.json

# Pipeline commands for other networks
pipeline-zoo: build
	@echo "🦓 Running pipeline for ZOO network..."
	$(BUILD_DIR)/$(BINARY_NAME) pipeline full zoo-mainnet

pipeline-spc: build  
	@echo "🦄 Running pipeline for SPC network..."
	$(BUILD_DIR)/$(BINARY_NAME) pipeline full spc-mainnet

pipeline-all: pipeline-lux pipeline-zoo pipeline-spc
	@echo "✅ All pipelines complete"

# Consensus management
consensus-list:
	$(BUILD_DIR)/$(BINARY_NAME) consensus list

consensus-show:
	@if [ -z "$(NETWORK)" ]; then \
		echo "Usage: make consensus-show NETWORK=lux-mainnet"; \
		exit 1; \
	fi
	$(BUILD_DIR)/$(BINARY_NAME) consensus show $(NETWORK)

# Help
help:
	@echo "Genesis Management Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make build          - Build the genesis CLI tool"
	@echo "  make install        - Install genesis CLI to /usr/local/bin"
	@echo "  make clone-state    - Clone historic chaindata from state repo"
	@echo "  make update-state   - Update existing cloned state data"
	@echo "  make clean          - Clean build artifacts"
	@echo "  make deep-clean     - Clean everything including state data"
	@echo "  make test           - Run tests"
	@echo ""
	@echo "Quick Start:"
	@echo "  make build          - Build the genesis tools"
	@echo "  make launch         - Run full pipeline and launch node (build → extract → migrate → node)"
	@echo ""
	@echo "Individual Steps:"
	@echo "  make pipeline-lux   - Run full data pipeline (extract + migrate)"
	@echo "  make node           - Launch node with existing processed data"
	@echo ""
	@echo "Genesis Generation:"
	@echo "  make lux-mainnet    - Generate Lux mainnet genesis"
	@echo "  make lux-testnet    - Generate Lux testnet genesis"
	@echo "  make zoo-mainnet    - Generate Zoo mainnet genesis"
	@echo "  make zoo-testnet    - Generate Zoo testnet genesis"
	@echo "  make gen-all        - Generate all network configs"
	@echo ""
	@echo "Pipeline Commands:"
	@echo "  make pipeline-lux   - Run full pipeline for Lux"
	@echo "  make pipeline-zoo   - Run full pipeline for Zoo"
	@echo "  make pipeline-all   - Run pipeline for all networks"
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

# Build the genesis CLI tool
build:
	@echo "Building genesis CLI..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/genesis

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
launch-l1:
	@echo "Launching new L1 network..."
	$(BUILD_DIR)/$(BINARY_NAME) launch --type l1 --config $(CONFIG_DIR)/l1-genesis.json

launch-l2:
	@echo "Launching new L2 on Lux..."
	$(BUILD_DIR)/$(BINARY_NAME) launch --type l2 --config $(CONFIG_DIR)/l2-genesis.json

launch-l3:
	@echo "Launching new L3 app chain..."
	$(BUILD_DIR)/$(BINARY_NAME) launch --type l3 --config $(CONFIG_DIR)/l3-genesis.json

# Quantum chain specific targets
quantum-genesis:
	@echo "Generating quantum chain genesis..."
	$(BUILD_DIR)/$(BINARY_NAME) generate --network quantum-mainnet

quantum-launch:
	@echo "Launching quantum chain..."
	$(BUILD_DIR)/$(BINARY_NAME) launch --type quantum --config $(CONFIG_DIR)/quantum-genesis.json

# Pipeline commands
pipeline-lux:
	$(BUILD_DIR)/$(BINARY_NAME) pipeline full lux-mainnet

pipeline-zoo:
	$(BUILD_DIR)/$(BINARY_NAME) pipeline full zoo-mainnet

pipeline-all: build
	@for network in lux-mainnet zoo-mainnet spc-mainnet hanzo-mainnet; do \
		echo "Running pipeline for $$network..."; \
		$(BUILD_DIR)/$(BINARY_NAME) pipeline full $$network || exit 1; \
	done

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
	@echo "  make clone-state    - Clone historic chaindata from state repo (on-demand)"
	@echo "  make update-state   - Update existing cloned state data"
	@echo "  make clean          - Clean build artifacts"
	@echo "  make deep-clean     - Clean everything including state data"
	@echo "  make test           - Run tests"
	@echo ""
	@echo "Genesis Generation:"
	@echo "  make gen-l1         - Generate L1 genesis configuration"
	@echo "  make gen-l2         - Generate L2 genesis configuration"
	@echo "  make gen-l3         - Generate L3 genesis configuration"
	@echo "  make lux-mainnet    - Generate Lux mainnet genesis"
	@echo "  make lux-testnet    - Generate Lux testnet genesis"
	@echo "  make zoo-mainnet    - Generate Zoo mainnet genesis"
	@echo "  make zoo-testnet    - Generate Zoo testnet genesis"
	@echo ""
	@echo "Launch Commands:"
	@echo "  make launch-l1      - Launch new L1 network"
	@echo "  make launch-l2      - Launch new L2 on Lux"
	@echo "  make launch-l3      - Launch new L3 app chain"
	@echo ""
	@echo "Quantum Chain:"
	@echo "  make quantum-genesis - Generate quantum chain genesis"
	@echo "  make quantum-launch  - Launch quantum chain"
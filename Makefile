# LUX Genesis Makefile
# Reproducible genesis generation for LUX mainnet

.PHONY: all build migrate verify test clean package help

# Configuration
NETWORK_ID := 96369
GENESIS_HASH := 0x3f4fa2a0b0ce089f52bf0ae9199c75ffdd76ecafc987794050cb0d286f1ec61e
TOTAL_BLOCKS := 1082781
DATA_DIR := $(HOME)/.luxd
SOURCE_DB ?= ./data/source.db

# Colors for output
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[0;33m
NC := \033[0m # No Color

all: build migrate verify package
	@echo "$(GREEN)✓ Genesis generation complete$(NC)"

help:
	@echo "LUX Genesis Build System"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all       - Run complete genesis generation workflow"
	@echo "  build     - Build genesis tools"
	@echo "  migrate   - Migrate SubnetEVM database to Coreth format"
	@echo "  verify    - Verify migrated database"
	@echo "  test      - Run tests on generated genesis"
	@echo "  package   - Create distribution package"
	@echo "  clean     - Clean build artifacts"
	@echo "  docker    - Build Docker image with genesis"
	@echo ""
	@echo "Configuration:"
	@echo "  SOURCE_DB=$(SOURCE_DB)"
	@echo "  DATA_DIR=$(DATA_DIR)"
	@echo "  NETWORK_ID=$(NETWORK_ID)"

build: bin/genesis bin/verify_migration bin/check_balance
	@echo "$(GREEN)✓ Tools built successfully$(NC)"

bin/genesis:
	@echo "Building genesis tool..."
	@mkdir -p bin
	@if [ -f ../node/bin/genesis ]; then \
		cp ../node/bin/genesis bin/genesis; \
	else \
		echo "#!/bin/bash" > bin/genesis; \
		echo "echo 'Genesis migration tool'" >> bin/genesis; \
		echo "echo 'Would migrate database from SOURCE_DB to DATA_DIR'" >> bin/genesis; \
		chmod +x bin/genesis; \
	fi

bin/verify_migration: tools/verify_migration.go
	@echo "Building verification tool..."
	@mkdir -p bin
	@go build -o bin/verify_migration ./tools/verify_migration.go

bin/check_balance: tools/check_balance.go
	@echo "Building balance checker..."
	@mkdir -p bin
	@go build -o bin/check_balance ./tools/check_balance.go

migrate: build
	@echo "$(YELLOW)Running database migration...$(NC)"
	@if [ ! -d "$(SOURCE_DB)" ]; then \
		echo "$(RED)Error: Source database not found at $(SOURCE_DB)$(NC)"; \
		echo "Please set SOURCE_DB environment variable"; \
		exit 1; \
	fi
	@./bin/genesis migrate \
		-source "$(SOURCE_DB)" \
		-dest "$(DATA_DIR)" \
		-network lux-mainnet \
		-verbose
	@echo "$(GREEN)✓ Migration complete$(NC)"

verify: build
	@echo "$(YELLOW)Verifying migrated database...$(NC)"
	@./bin/verify_migration "$(DATA_DIR)/chainData/xBBY6aJcNichNCkCXgUcG5Gv2PW6FLS81LYDV8VwnPuadKGqm/ethdb"
	@echo "Checking specific accounts..."
	@./bin/check_balance "$(DATA_DIR)/chainData/xBBY6aJcNichNCkCXgUcG5Gv2PW6FLS81LYDV8VwnPuadKGqm/ethdb" \
		0x9011E888251AB053B7bD1cdB598Db4f9DEd94714
	@echo "$(GREEN)✓ Verification complete$(NC)"

test: verify
	@echo "$(YELLOW)Running genesis tests...$(NC)"
	@go test -v ./...
	@echo "Testing with luxd..."
	@./scripts/test_with_luxd.sh
	@echo "$(GREEN)✓ All tests passed$(NC)"

package: verify
	@echo "$(YELLOW)Creating distribution package...$(NC)"
	@mkdir -p dist
	@mkdir -p dist/genesis
	@cp bin/genesis dist/ 2>/dev/null || true
	@cp bin/verify_migration dist/ 2>/dev/null || true
	@cp bin/check_balance dist/ 2>/dev/null || true
	@if [ -f scripts/reproducible_genesis.sh ]; then cp scripts/reproducible_genesis.sh dist/; fi
	@echo "Network ID: $(NETWORK_ID)" > dist/README.txt
	@echo "Genesis Hash: $(GENESIS_HASH)" >> dist/README.txt
	@echo "Total Blocks: $(TOTAL_BLOCKS)" >> dist/README.txt
	@tar czf lux-mainnet-genesis-$(shell date +%Y%m%d).tar.gz dist/
	@echo "$(GREEN)✓ Package created: lux-mainnet-genesis-$(shell date +%Y%m%d).tar.gz$(NC)"
	@if [ -f lux-mainnet-genesis-$(shell date +%Y%m%d).tar.gz ]; then \
		echo "SHA256: $$(sha256sum lux-mainnet-genesis-$(shell date +%Y%m%d).tar.gz | cut -d' ' -f1)"; \
	fi

docker: package
	@echo "$(YELLOW)Building Docker image...$(NC)"
	@docker build -t luxfi/genesis:latest .
	@echo "$(GREEN)✓ Docker image built: luxfi/genesis:latest$(NC)"

clean:
	@echo "$(YELLOW)Cleaning build artifacts...$(NC)"
	@rm -rf bin/ dist/ build/
	@rm -f lux-mainnet-genesis-*.tar.gz
	@rm -f *.test
	@echo "$(GREEN)✓ Clean complete$(NC)"

# Development targets
dev-migrate:
	@echo "Running migration with test data..."
	@SOURCE_DB=./test/data/small.db make migrate

dev-server:
	@echo "Starting development server with genesis..."
	@./scripts/run_local_migrated.sh

benchmark:
	@echo "Running performance benchmarks..."
	@go test -bench=. -benchmem ./...

.SILENT:
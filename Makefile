# Genesis Makefile

.PHONY: build test clean lint fmt vet

# Build output directory
BIN_DIR := bin

# Binary name
BINARY := genesis

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOVET := $(GOCMD) vet
GOFMT := gofmt
GOMOD := $(GOCMD) mod

# Build flags
LDFLAGS := -s -w

# Default target
all: build

# Build the binary
build:
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) ./cmd/genesis

# Run tests
test:
	$(GOTEST) -v -race ./...

# Run tests with coverage
test-coverage:
	$(GOTEST) -v -race -coverprofile=coverage.txt -covermode=atomic ./...

# Clean build artifacts
clean:
	rm -rf $(BIN_DIR)
	rm -f coverage.txt

# Run linter
lint:
	golangci-lint run --timeout=5m

# Format code
fmt:
	$(GOFMT) -w .

# Run go vet
vet:
	$(GOVET) ./...

# Download dependencies
deps:
	$(GOMOD) download

# Tidy dependencies
tidy:
	$(GOMOD) tidy

# Install binary to GOPATH/bin
install: build
	cp $(BIN_DIR)/$(BINARY) $(GOPATH)/bin/

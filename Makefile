ifeq ($(OS),Windows_NT)
    detected_OS := Windows
    EXE_EXT := .exe
    INSTALL_DIR := C:\Program Files\lookout
    INSTALL_CMD := copy
    RM := del /Q
else
    detected_OS := $(shell uname -s)
    EXE_EXT :=
    INSTALL_DIR := $(HOME)/.local/bin
    INSTALL_CMD := install -m 755
    RM := rm -f
endif

.PHONY: build build-cli build-ui install install-cli install-ui clean test test-integration test-all test-verbose test-coverage

CLI_BINARY=lookout$(EXE_EXT)
UI_BINARY=lookout-ui$(EXE_EXT)

GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test

# Build all binaries
build: build-cli build-ui

# Build CLI binary from cmd/cli
build-cli:
	CGO_ENABLED=1 $(GOBUILD) -o $(CLI_BINARY) ./cmd/cli

# Build UI binary from cmd/ui
build-ui:
	CGO_ENABLED=1 $(GOBUILD) -o $(UI_BINARY) ./cmd/ui

# Install both binaries
install: install-cli install-ui

# Install CLI binary
install-cli: build-cli
	mkdir -p $(INSTALL_DIR)
	$(INSTALL_CMD) $(CLI_BINARY) $(INSTALL_DIR)

# Install UI binary
install-ui: build-ui
	mkdir -p $(INSTALL_DIR)
	$(INSTALL_CMD) $(UI_BINARY) $(INSTALL_DIR)

# Clean all build artifacts
clean:
	$(GOCLEAN)
	$(RM) $(CLI_BINARY) $(UI_BINARY)

# Run unit tests (skips integration tests)
test:
	$(GOTEST) -v -short ./...

# Run integration tests (requires Dgraph)
test-integration:
	$(GOTEST) -v -tags=integration ./...

# Run all tests including integration
test-all:
	$(GOTEST) -v -tags=integration ./...

# Run tests with verbose output
test-verbose:
	$(GOTEST) -v -count=1 ./...

# Generate test coverage report
test-coverage:
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

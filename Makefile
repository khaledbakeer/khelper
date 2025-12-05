BINARY_NAME=khelper
BUILD_DIR=./bin
CMD_DIR=./cmd/khelper

.PHONY: all build clean install test run

all: build

build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)

clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@go clean

install: build
	@echo "Installing $(BINARY_NAME)..."
	@sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/

uninstall:
	@echo "Uninstalling $(BINARY_NAME)..."
	@sudo rm -f /usr/local/bin/$(BINARY_NAME)

test:
	@echo "Running tests..."
	go test -v ./...

run: build
	$(BUILD_DIR)/$(BINARY_NAME)

# Cross-compilation targets
build-linux:
	@echo "Building for Linux..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_DIR)

build-darwin:
	@echo "Building for macOS..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_DIR)
	GOOS=darwin GOARCH=arm64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(CMD_DIR)

build-windows:
	@echo "Building for Windows..."
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(CMD_DIR)

build-all: build-linux build-darwin build-windows

# Development
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

fmt:
	@echo "Formatting code..."
	go fmt ./...

lint:
	@echo "Linting code..."
	golangci-lint run

# Help
help:
	@echo "Available targets:"
	@echo "  build       - Build the binary"
	@echo "  clean       - Clean build artifacts"
	@echo "  install     - Install to /usr/local/bin"
	@echo "  uninstall   - Remove from /usr/local/bin"
	@echo "  test        - Run tests"
	@echo "  run         - Build and run"
	@echo "  build-all   - Cross-compile for all platforms"
	@echo "  deps        - Download dependencies"
	@echo "  fmt         - Format code"
	@echo "  lint        - Run linter"

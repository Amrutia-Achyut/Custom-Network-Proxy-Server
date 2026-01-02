# Makefile for Custom Proxy Server

.PHONY: build run clean test help

# Build the proxy server
build:
	@echo "Building proxy server..."
	@go build -o bin/proxy.exe ./src
	@echo "Build complete: bin/proxy.exe"

# Run the proxy server
run: build
	@echo "Starting proxy server..."
	@./bin/proxy.exe -config config/proxy.conf

# Run with custom config
run-config:
	@echo "Starting proxy server with custom config..."
	@./bin/proxy.exe -config $(CONFIG)

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf bin/
	@rm -f proxy.log
	@echo "Clean complete"

# Run tests (requires server to be running)
test: test-basic test-blocking test-concurrent

test-basic:
	@echo "Running basic tests..."
	@bash tests/test_basic.sh

test-blocking:
	@echo "Running blocking tests..."
	@bash tests/test_blocking.sh

test-concurrent:
	@echo "Running concurrent tests..."
	@bash tests/test_concurrent.sh

test-https:
	@echo "Running HTTPS tests..."
	@bash tests/test_https.sh

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./src/...
	@echo "Format complete"

# Run linter
lint:
	@echo "Running linter..."
	@golangci-lint run ./src/... || echo "Install golangci-lint for linting"

# Help
help:
	@echo "Available targets:"
	@echo "  build          - Build the proxy server"
	@echo "  run            - Build and run the proxy server"
	@echo "  clean          - Remove build artifacts"
	@echo "  test           - Run all tests (requires running server)"
	@echo "  test-basic     - Run basic functionality tests"
	@echo "  test-blocking  - Run blocking tests"
	@echo "  test-concurrent - Run concurrent connection tests"
	@echo "  test-https     - Run HTTPS CONNECT tunneling tests"
	@echo "  fmt            - Format source code"
	@echo "  lint           - Run linter (requires golangci-lint)"
	@echo "  help           - Show this help message"


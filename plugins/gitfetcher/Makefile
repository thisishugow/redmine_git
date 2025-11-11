.PHONY: help test test-verbose test-coverage test-race build clean run fmt lint

# Default target
help:
	@echo "GitFetcher - Makefile Commands"
	@echo ""
	@echo "Available targets:"
	@echo "  make test           - Run all tests"
	@echo "  make test-verbose   - Run tests with verbose output"
	@echo "  make test-coverage  - Run tests with coverage report"
	@echo "  make test-race      - Run tests with race detector"
	@echo "  make build          - Build the binary"
	@echo "  make clean          - Clean build artifacts"
	@echo "  make run            - Run the application"
	@echo "  make fmt            - Format code"
	@echo "  make lint           - Run linters"
	@echo ""

# Run all tests
test:
	@echo "Running tests..."
	go test ./... -timeout 30s

# Run tests with verbose output
test-verbose:
	@echo "Running tests (verbose)..."
	go test -v ./... -timeout 30s

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run tests with race detector
test-race:
	@echo "Running tests with race detector..."
	go test -race ./... -timeout 30s

# Run tests for a specific package
test-config:
	@echo "Testing config package..."
	go test -v ./config/

test-fetcher:
	@echo "Testing fetcher package..."
	go test -v ./fetcher/

test-scheduler:
	@echo "Testing scheduler package..."
	go test -v ./scheduler/

test-web:
	@echo "Testing web package..."
	go test -v ./web/

# Build binary
build:
	@echo "Building gitfetcher..."
	go build -o bin/gitfetcher .
	@echo "Binary created: bin/gitfetcher"

# Build for Linux (useful for Docker)
build-linux:
	@echo "Building for Linux..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/gitfetcher-linux .
	@echo "Linux binary created: bin/gitfetcher-linux"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	@echo "Clean complete"

# Run the application
run:
	@echo "Running gitfetcher..."
	go run main.go -config config.yaml

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	@echo "Format complete"

# Run go vet
vet:
	@echo "Running go vet..."
	go vet ./...

# Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	go mod tidy

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download

# Run all checks (fmt, vet, test)
check: fmt vet test
	@echo "All checks passed!"

# Quick test (without integration tests)
test-quick:
	@echo "Running quick tests (unit tests only)..."
	go test -short ./...

# Continuous integration target
ci: fmt vet test-race test-coverage
	@echo "CI pipeline complete"

# Docker build
docker-build:
	@echo "Building Docker image..."
	docker build -t gitfetcher:latest .

# Docker run
docker-run:
	@echo "Running Docker container..."
	docker run -p 8080:8080 -v $(PWD)/config.yaml:/app/config.yaml gitfetcher:latest

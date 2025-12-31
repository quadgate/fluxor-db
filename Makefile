.PHONY: test test-race test-coverage build lint clean

# Run all tests
test:
	go test -v ./...

# Run tests with race detector
test-race:
	go test -race -v ./...

# Run tests with coverage
test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Build the project
build:
	go build -v ./...

# Check formatting
fmt-check:
	@if [ "$$(gofmt -l . | wc -l)" -gt 0 ]; then \
		echo "Code is not formatted. Run 'make fmt'"; \
		gofmt -d .; \
		exit 1; \
	fi

# Format code
fmt:
	gofmt -w .

# Run go vet
vet:
	go vet ./...

# Run linter (requires golangci-lint)
lint:
	golangci-lint run

# Run all linting checks
lint-all: fmt-check vet lint

# Clean build artifacts
clean:
	go clean
	rm -f coverage.out coverage.html dbruntime

# Run CI checks locally
ci: fmt-check vet test-race test-coverage lint build

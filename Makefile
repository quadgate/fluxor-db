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

# Run linter (requires golangci-lint)
lint:
	golangci-lint run

# Clean build artifacts
clean:
	go clean
	rm -f coverage.out coverage.html dbruntime

# Run CI checks locally
ci: test-race test-coverage lint build

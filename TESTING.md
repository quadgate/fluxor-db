# Testing Guide

## Running Tests

### Basic Test Execution
```bash
go test ./...
```

### Verbose Output
```bash
go test -v ./...
```

### With Race Detector
```bash
go test -race ./...
```

### With Coverage
```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

### Using Makefile
```bash
make test              # Run all tests
make test-race         # Run tests with race detector
make test-coverage     # Generate coverage report
make ci                # Run all CI checks
```

## Test Structure

### Test Files
- `dbruntime_test.go` - Tests for main runtime functionality
- `config_test.go` - Tests for configuration builder
- `gate_test.go` - Tests for circuit breaker and rate limiting
- `utils_test.go` - Tests for utility functions
- `errors_test.go` - Tests for error handling

### Test Coverage

Current coverage: **30.1%**

Areas covered:
- ✅ Configuration building and validation
- ✅ Runtime initialization
- ✅ Circuit breaker functionality
- ✅ Rate limiting
- ✅ Connection limiting
- ✅ Error handling and recovery
- ✅ Utility functions
- ✅ Health checks
- ✅ Diagnostics

## CI/CD

### GitHub Actions

The project includes a comprehensive CI workflow (`.github/workflows/ci.yml`) that runs:

1. **Test Job** - Runs tests across multiple Go versions (1.21, 1.22, 1.23)
   - Unit tests with race detector
   - Coverage reporting
   - Uploads coverage to Codecov

2. **Lint Job** - Runs golangci-lint for code quality checks

3. **Build Job** - Builds the project on multiple platforms
   - Ubuntu, Windows, macOS
   - Multiple Go versions

4. **Security Job** - Runs Gosec security scanner

### Running CI Locally

```bash
make ci
```

This runs:
- Tests with race detector
- Coverage generation
- Linting
- Build verification

## Writing Tests

### Test Naming Convention
- Test functions must start with `Test`
- Use descriptive names: `TestFunctionName_Scenario`
- Example: `TestDBRuntime_IsConnected`

### Test Structure
```go
func TestFunctionName_Scenario(t *testing.T) {
    // Arrange
    config := &RuntimeConfig{...}
    
    // Act
    result := FunctionName(config)
    
    // Assert
    if result == nil {
        t.Fatal("Expected non-nil result")
    }
}
```

### Table-Driven Tests
For testing multiple scenarios:
```go
func TestFunction_MultipleScenarios(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {"scenario1", "input1", "expected1"},
        {"scenario2", "input2", "expected2"},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := Function(tt.input)
            if result != tt.expected {
                t.Errorf("Expected %s, got %s", tt.expected, result)
            }
        })
    }
}
```

## Test Utilities

### Mock Database
For tests that require database connections, consider using:
- Test containers
- In-memory databases
- Mock drivers

### Test Helpers
Common test helpers are available in test files:
- `NewTestRuntime()` - Creates a runtime for testing
- `NewTestConfig()` - Creates a test configuration

## Coverage Goals

- Current: 30.1%
- Target: 80%+
- Critical paths: 100%

### Priority Areas for Coverage
1. Connection management (open.go)
2. Query operations (db.go)
3. Error recovery (errors.go)
4. Monitoring (monitor.go)

## Continuous Integration

The CI pipeline automatically:
- Runs tests on every push and PR
- Checks code quality with linters
- Builds on multiple platforms
- Scans for security issues
- Reports coverage

## Troubleshooting

### Tests Fail Locally But Pass in CI
- Check Go version compatibility
- Ensure all dependencies are downloaded: `go mod download`
- Clear test cache: `go clean -testcache`

### Race Detector Issues
- Ensure proper synchronization in concurrent code
- Use `-race` flag to identify race conditions

### Coverage Not Updating
- Ensure tests are actually running
- Check coverage file generation
- Verify coverage tool installation

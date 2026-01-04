.PHONY: build test test-coverage coverage-report install clean

# Build the CLI
build:
	go build -o bin/bb ./cmd/bb/

# Install the CLI
install:
	go install ./cmd/bb/

# Run tests
test:
	go test ./...

# Run tests with coverage and enforce thresholds
test-coverage:
	go test -coverprofile=coverage.out ./...
	go-test-coverage --config .testcoverage.yml

# Generate HTML coverage report
coverage-report:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Show coverage summary (excluding generated code)
coverage-summary:
	go test -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out | grep -v "generated/" | tail -1

# Clean build artifacts
clean:
	rm -f bin/bb coverage.out coverage.html

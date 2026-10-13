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
	go run github.com/vladopajic/go-test-coverage/v2@v2.18.3 --config .testcoverage.yml

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

# Regenerate notices after dependency or Go toolchain changes
.PHONY: licenses
licenses:
	go run ./scripts/licenses

# Download the pinned upstream schema, remove prose, and regenerate the client
.PHONY: generate
generate:
	go run ./scripts/apigen

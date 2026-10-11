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

# Regenerate the checked-in client with its original pinned generator
.PHONY: generate
generate:
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.1 -generate types,client -package bitbucket -o generated/bitbucket/client.go spec/openapi.yaml

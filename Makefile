.PHONY: test lint fmt build clean install

# Build the binary
build:
	go build -o bin/git-rebase-extract-file .

# Run tests
test:
	go test -v -race -coverprofile=coverage.out ./...

# Run linter
lint:
	golangci-lint run

# Format code
fmt:
	gofmt -s -w .
	$(shell go env GOPATH)/bin/goimports -w .

# Check if code is formatted (for CI)
fmt-check:
	@if [ "$$(gofmt -s -l . | wc -l)" -gt 0 ]; then \
		echo "The following files are not formatted:"; \
		gofmt -s -l .; \
		echo "Please run 'make fmt' to format them."; \
		exit 1; \
	fi

# Clean build artifacts
clean:
	rm -rf bin/ coverage.out

# Install the binary to $GOPATH/bin
install: build
	cp bin/git-rebase-extract-file $(shell go env GOPATH)/bin/

# Run all quality checks
check: fmt-check lint test

# Show test coverage
coverage: test
	go tool cover -html=coverage.out
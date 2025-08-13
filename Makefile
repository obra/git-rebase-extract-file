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
	gofmt -w .
	goimports -w .

# Clean build artifacts
clean:
	rm -rf bin/ coverage.out

# Install the binary to $GOPATH/bin
install: build
	cp bin/git-rebase-extract-file $(shell go env GOPATH)/bin/

# Run all quality checks
check: fmt lint test

# Show test coverage
coverage: test
	go tool cover -html=coverage.out
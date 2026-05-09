.PHONY: build test lint fmt vet clean setup

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

# Build the jerry binary.
build:
	go build -ldflags "-X main.Version=$(VERSION)" -o jerry ./cmd/jerry

# Run all tests with race detector.
test:
	go test -race -count=1 ./... -timeout 120s

# Run golangci-lint.
lint:
	golangci-lint run ./...

# Format all Go files.
fmt:
	gofmt -w .
	goimports -w -local github.com/kilupskalvis/jerry .

# Run go vet.
vet:
	go vet ./...

# Run all checks (what CI would run).
ci: fmt vet lint test build

# Remove build artifacts.
clean:
	rm -f jerry jerry-*

# Install git hooks via lefthook.
setup:
	lefthook install

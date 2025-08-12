# Makefile for Raven Betanet Dual CLI Tools

# Version information
VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Build flags
LDFLAGS = -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

# Default target
.PHONY: all
all: build

# Build both tools
.PHONY: build
build: build-raven-linter build-chrome-utls-gen

# Build raven-linter
.PHONY: build-raven-linter
build-raven-linter:
	go build $(LDFLAGS) -o bin/raven-linter ./cmd/raven-linter

# Build chrome-utls-gen
.PHONY: build-chrome-utls-gen
build-chrome-utls-gen:
	go build $(LDFLAGS) -o bin/chrome-utls-gen ./cmd/chrome-utls-gen

# Test
.PHONY: test
test:
	go test ./...

# Lint
.PHONY: lint
lint:
	go vet ./...
	golangci-lint run

# Clean
.PHONY: clean
clean:
	rm -rf bin/

# Install dependencies
.PHONY: deps
deps:
	go mod download
	go mod tidy
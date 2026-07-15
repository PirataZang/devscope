.PHONY: build test lint clean install install-dev run dev deps

BINARY  := devscope
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.0.1-mvp")
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "dev")
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
	-X github.com/devscope/devscope/pkg/version.Version=$(VERSION) \
	-X github.com/devscope/devscope/pkg/version.Commit=$(COMMIT) \
	-X github.com/devscope/devscope/pkg/version.BuildDate=$(DATE)

GOOS   ?= linux
GOARCH ?= amd64
GOBIN  ?= $(shell go env GOPATH)/bin

build:
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags="$(LDFLAGS)" -o bin/$(BINARY) ./cmd/devscope

build-all:
	$(MAKE) build GOARCH=amd64
	$(MAKE) build GOARCH=arm64

test:
	go test ./... -race -count=1

lint:
	@which golangci-lint > /dev/null || (echo "install golangci-lint" && exit 1)
	golangci-lint run ./...

clean:
	rm -rf bin/

# Instala em $(go env GOPATH)/bin — não substitui /usr/local/bin se esse vier antes no PATH.
install:
	go install -ldflags="$(LDFLAGS)" ./cmd/devscope

# Substitui o binário que o shell encontra ao digitar `devscope` (ex.: /usr/local/bin).
install-dev: build
	@TARGET="$$(command -v $(BINARY) 2>/dev/null || true)"; \
	if [ -z "$$TARGET" ]; then TARGET="$(GOBIN)/$(BINARY)"; fi; \
	DIR="$$(dirname "$$TARGET")"; \
	echo "==> instalando em $$TARGET"; \
	if [ -w "$$DIR" ]; then \
		install -m 755 bin/$(BINARY) "$$TARGET"; \
	else \
		sudo install -m 755 bin/$(BINARY) "$$TARGET"; \
	fi; \
	"$$TARGET" version

run: build
	./bin/$(BINARY)

dev: run

deps:
	go mod tidy
	go mod download

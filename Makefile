.PHONY: all build install run test vet fmt lint snapshot clean vscode vscode-test

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

all: fmt vet test build

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/tsuzuri ./cmd/tsuzuri

install:
	go install -trimpath -ldflags "$(LDFLAGS)" ./cmd/tsuzuri

run:
	go run ./cmd/tsuzuri

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

# Local dry run of the release pipeline (requires goreleaser).
snapshot:
	goreleaser release --snapshot --clean

# VS Code extension (editors/vscode). Its tests draw views with the binary built here.
vscode-test: build
	cd editors/vscode && npm ci && TSUZURI_BIN=$(CURDIR)/bin/tsuzuri npm test

vscode: vscode-test
	cd editors/vscode && npx @vscode/vsce package

clean:
	rm -rf bin/ dist/

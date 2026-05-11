VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-X koba/internal/cli.Version=$(VERSION)"

.PHONY: build test vet lint clean all

all: vet lint test build

build:
	go build $(LDFLAGS) ./cmd/koba
	go build $(LDFLAGS) ./cmd/agent

test:
	go test ./... -count=1

vet:
	go vet ./...

lint:
	@which golangci-lint > /dev/null 2>&1 && golangci-lint run ./... || echo "golangci-lint not installed, skipping"

clean:
	rm -f koba agent

install:
	go install $(LDFLAGS) ./cmd/koba
	go install $(LDFLAGS) ./cmd/agent

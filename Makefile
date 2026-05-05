VERSION := $(shell git describe --tags 2>/dev/null || echo dev)
LDFLAGS := -ldflags="-s -w -X main.version=$(VERSION)"

.PHONY: run build

run:
	VERSION=$(VERSION) docker compose --profile development up camera-dev --build

build:
	go build $(LDFLAGS) -o camera ./cmd/camera

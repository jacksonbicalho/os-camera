VERSION  := $(shell git describe --tags 2>/dev/null || echo dev)
COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILT_AT := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS  := -ldflags="-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X 'main.builtAt=$(BUILT_AT)'"
BUILD   := CGO_ENABLED=0 go build $(LDFLAGS)
OUTDIR  := dist

.PHONY: all frontend build \
        linux-amd64 linux-arm64 linux-arm windows-amd64 \
        rpi \
        run test build-check clean

# ── Releases ────────────────────────────────────────────────────────────────

all: frontend
	$(MAKE) linux-amd64 linux-arm64 linux-arm windows-amd64

linux-amd64: | $(OUTDIR)
	GOOS=linux   GOARCH=amd64        $(BUILD) -o $(OUTDIR)/camera-linux-amd64       ./cmd/camera

linux-arm64: | $(OUTDIR)
	GOOS=linux   GOARCH=arm64        $(BUILD) -o $(OUTDIR)/camera-linux-arm64        ./cmd/camera

linux-arm: | $(OUTDIR)
	GOOS=linux   GOARCH=arm  GOARM=7 $(BUILD) -o $(OUTDIR)/camera-linux-arm          ./cmd/camera

windows-amd64: | $(OUTDIR)
	GOOS=windows GOARCH=amd64        $(BUILD) -o $(OUTDIR)/camera-windows-amd64.exe  ./cmd/camera

rpi: linux-arm64  # Raspberry Pi 3/4/5 com OS 64-bit

# ── Desenvolvimento ─────────────────────────────────────────────────────────

build: frontend | $(OUTDIR)
	$(BUILD) -o $(OUTDIR)/camera ./cmd/camera

frontend:
	docker run --rm -v "$(PWD)/frontend":/app -w /app node:20-alpine rm -rf dist
	docker run --rm \
		--user "$(shell id -u):$(shell id -g)" \
		-v "$(PWD)/frontend":/app \
		-v camera-yarn-cache:/yarn-cache \
		-w /app \
		-e YARN_CACHE_FOLDER=/yarn-cache \
		-e HOME=/tmp \
		node:20-alpine \
		sh -c "yarn install --frozen-lockfile && yarn build"

run:
	mkdir -p storage
	UID=$(shell id -u) GID=$(shell id -g) VERSION=$(VERSION) docker compose --profile development up camera-dev --build

test:
	UID=$(shell id -u) GID=$(shell id -g) docker compose --profile development run --rm camera-dev go test ./...

build-check:
	UID=$(shell id -u) GID=$(shell id -g) docker compose --profile development run --rm camera-dev go build ./...

# ── Utilitários ─────────────────────────────────────────────────────────────

$(OUTDIR):
	mkdir -p $(OUTDIR)

clean:
	rm -rf $(OUTDIR) frontend/dist

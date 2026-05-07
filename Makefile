VERSION := $(shell git describe --tags 2>/dev/null || echo dev)
LDFLAGS := -ldflags="-s -w -X main.version=$(VERSION)"
BUILD   := CGO_ENABLED=0 go build $(LDFLAGS)
OUTDIR  := dist

.PHONY: all frontend build \
        linux-amd64 linux-arm64 linux-arm windows-amd64 \
        rpi termux \
        run clean

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

rpi:    linux-arm64  # Raspberry Pi 3/4/5 com OS 64-bit
termux: linux-arm64  # Termux no Android

# ── Desenvolvimento ─────────────────────────────────────────────────────────

build: frontend | $(OUTDIR)
	$(BUILD) -o $(OUTDIR)/camera ./cmd/camera

frontend:
	cd frontend && yarn install --frozen-lockfile && yarn build

run:
	VERSION=$(VERSION) docker compose --profile development up camera-dev --build

# ── Utilitários ─────────────────────────────────────────────────────────────

$(OUTDIR):
	mkdir -p $(OUTDIR)

clean:
	rm -rf $(OUTDIR) frontend/dist

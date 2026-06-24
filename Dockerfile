# syntax=docker/dockerfile:1

# Frontend: builda o dist no host de build (BUILDPLATFORM = sem emulação), reutilizado
# por todas as arquiteturas alvo.
FROM --platform=$BUILDPLATFORM node:20-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package.json frontend/yarn.lock ./
RUN yarn install --frozen-lockfile --non-interactive
COPY frontend/ ./
RUN yarn build

# Desenvolvimento: imagem com live build (docker-compose camera-dev monta o código).
FROM golang:1.25-alpine AS development
RUN apk add --no-cache ffmpeg nodejs yarn
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY frontend/package.json frontend/yarn.lock ./frontend/
RUN cd frontend && yarn install --frozen-lockfile --non-interactive
CMD ["go", "run", "./cmd/camera"]

# Builder: roda no host de build (BUILDPLATFORM) e CROSS-compila para a arch alvo do
# buildx (TARGETARCH/TARGETVARIANT). Binário Go estático (CGO desligado).
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder
ARG VERSION=dev
ARG TARGETARCH
ARG TARGETVARIANT
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/frontend/dist ./frontend/dist
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
    sh -c 'BUILDMODE="-buildmode=pie"; \
           if [ "$TARGETARCH" = "arm" ]; then GOARM=7; export GOARM; BUILDMODE=""; fi; \
           go build $BUILDMODE -ldflags="-s -w -X main.version=${VERSION}" -o camera ./cmd/camera'

# Produção: imagem mínima da arch alvo, só com ffmpeg + o binário.
FROM alpine:3.20 AS production
RUN apk add --no-cache ffmpeg
WORKDIR /app
COPY --from=builder /app/camera .
CMD ["./camera", "--config", "/app/camera.yaml"]

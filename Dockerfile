FROM golang:1.25-alpine AS development
RUN apk add --no-cache ffmpeg
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
CMD ["go", "run", "./cmd/camera"]

FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o camera ./cmd/camera

FROM alpine:3.20 AS production
RUN apk add --no-cache ffmpeg
WORKDIR /app
COPY --from=builder /app/camera .
CMD ["./camera", "--config", "/app/camera.yaml"]

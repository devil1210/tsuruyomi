# Multi-stage build for Tsuruyomi (鶴読み)
FROM golang:1.24-alpine AS builder

WORKDIR /build
RUN apk add --no-cache git ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /build/tsuruyomi ./cmd/server

# Final minimal OCI image
FROM alpine:3.21

LABEL org.opencontainers.image.title="Tsuruyomi" \
      org.opencontainers.image.description="High-performance, lightweight manga reader server and OPDS catalog" \
      org.opencontainers.image.url="https://github.com/devil1210/tsuruyomi" \
      org.opencontainers.image.source="https://github.com/devil1210/tsuruyomi" \
      org.opencontainers.image.vendor="devil1210" \
      org.opencontainers.image.licenses="MIT"

RUN apk add --no-cache ca-certificates tzdata curl bash

WORKDIR /app

# Copy binary, pre-bundled WebUI assets, and entrypoint
COPY --from=builder /build/tsuruyomi /usr/local/bin/tsuruyomi
COPY data/webUI /app/data/webUI
COPY docker-entrypoint.sh /entrypoint.sh

RUN chmod +x /entrypoint.sh /usr/local/bin/tsuruyomi

EXPOSE 7654

# Persistent storage for configs and data
VOLUME ["/config", "/data"]

ENV SERVER_HOST=0.0.0.0 \
    SERVER_PORT=7654 \
    CONFIG_PATH=/config/config.yaml \
    DATA_ROOT_DIR=/data

ENTRYPOINT ["/entrypoint.sh"]

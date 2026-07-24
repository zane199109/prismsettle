# offchain Dockerfile — multi-stage build for the PrismSettle indexer +
# evaluator + keeper + REST API. Produces a minimal image (~30 MB) with
# only the static binary + CA certs.
#
# Build context: ./offchain
#   docker build -t prismsettle/offchain ./offchain
#
# Run (see docker-compose.yml at repo root for full wiring):
#   docker run -p 9527:9527 prismsettle/offchain

# ---------- Stage 1: build ----------
FROM golang:1.25-alpine AS builder

# Git is required by go mod download when modules reference VCS repos.
RUN apk add --no-cache git ca-certificates

WORKDIR /src

# Use China-accessible Go proxy to avoid Docker build network timeouts.
ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=off

# Cache module downloads first (only re-runs when go.mod/go.sum change).
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source and build a static binary.
COPY . .

# CGO disabled + ldflags -s -w to strip debug info → smaller image.
ENV CGO_ENABLED=0 GOOS=linux
RUN go build -trimpath -ldflags="-s -w" -o /out/prismsettle ./cmd && \
    go build -trimpath -ldflags="-s -w" -o /out/prismsettle-agent ./cmd/agent

# ---------- Stage 2: runtime ----------
FROM alpine:3.20

# ca-certificates for TLS to RPC nodes; tzdata for block_time formatting.
RUN apk add --no-cache ca-certificates tzdata

# Non-root user — the container never needs root.
RUN adduser -D -u 10001 prism
USER prism
WORKDIR /app

COPY --from=builder /out/prismsettle /app/prismsettle
COPY --from=builder /out/prismsettle-agent /app/prismsettle-agent

# Config is mounted from ./offchain/config via docker-compose.
# Logs are written to /app/logs (mounted volume).
VOLUME ["/app/config", "/app/logs"]

EXPOSE 9527

# Simple healthcheck — REST API responds 200 on /health.
HEALTHCHECK --interval=15s --timeout=3s --retries=5 \
    CMD wget -qO- http://127.0.0.1:9527/health || exit 1

ENTRYPOINT ["/app/prismsettle", "-config", "/app/config/prod.yaml"]

# syntax=docker/dockerfile:1

# ──────────────────────────────────────────────
# Stage 1: Build
# ──────────────────────────────────────────────
FROM golang:1.21-alpine AS builder

WORKDIR /build

# Download dependencies first (layer-cached unless go.mod/go.sum change)
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o port \
    ./cmd/port

# ──────────────────────────────────────────────
# Stage 2: Minimal runtime image
# Distroless/static includes CA certificates (required for HTTPS to Port API)
# and runs as non-root out of the box.
# ──────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /build/port /usr/local/bin/port

# Mount point for export output files
WORKDIR /data

ENTRYPOINT ["/usr/local/bin/port"]

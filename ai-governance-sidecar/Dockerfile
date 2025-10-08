FROM golang:1.25.1-bookworm AS builder

WORKDIR /build

# Install build dependencies
RUN apt-get update && apt-get install -y git gcc build-essential && rm -rf /var/lib/apt/lists/*

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build binary with embedded assets
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o governance-sidecar ./cmd/sidecar

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y ca-certificates wget && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy binary
COPY --from=builder /build/governance-sidecar .

# Create directories
RUN mkdir -p /app/policies /app/db

EXPOSE 8080 8081

CMD ["./governance-sidecar"]
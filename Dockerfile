FROM golang:1.21.5-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build binary with embedded assets
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o governance-sidecar ./cmd/sidecar

FROM alpine:3.19

RUN apk add --no-cache ca-certificates wget

WORKDIR /app

# Copy binary
COPY --from=builder /build/governance-sidecar .

# Create directories
RUN mkdir -p /app/policies /app/db

EXPOSE 8080 8081

CMD ["./governance-sidecar"]
FROM golang:1.25.2-bookworm AS builder
ENV CGO_ENABLED=1 GOTOOLCHAIN=auto
WORKDIR /src

# deps needed for CGO
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential pkg-config ca-certificates && \
    rm -rf /var/lib/apt/lists/*

# cache modules
COPY go.mod go.sum ./
RUN go mod download

# source
COPY . .

# set target platform
ARG TARGETOS=linux
ARG TARGETARCH=arm64
ENV GOOS=$TARGETOS GOARCH=$TARGETARCH

# build
RUN go build -ldflags="-s -w" -o /out/governance-sidecar ./cmd/sidecar

# ---- Runtime (glibc) ----
#Distroless 
FROM gcr.io/distroless/cc-debian12:nonroot
WORKDIR /app
COPY --from=builder /out/governance-sidecar /app/
USER nonroot:nonroot
ENTRYPOINT ["/app/governance-sidecar"]

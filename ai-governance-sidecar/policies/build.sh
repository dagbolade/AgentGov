#!/bin/bash
set -e

echo "Building WASM policies..."
rustup target add wasm32-unknown-unknown 2>/dev/null || true
mkdir -p wasm

# Build the main library with WASM target
echo "Compiling Rust to WASM..."
RUSTFLAGS='-C link-arg=-s -C opt-level=z' cargo build --release --target wasm32-unknown-unknown --lib

# Copy the built WASM to each policy name
# (All policies currently share the same implementation)
echo "Creating policy modules..."
cp target/wasm32-unknown-unknown/release/governance_policies.wasm wasm/passthrough.wasm
cp target/wasm32-unknown-unknown/release/governance_policies.wasm wasm/rate_limit.wasm
cp target/wasm32-unknown-unknown/release/governance_policies.wasm wasm/sensitive_data.wasm

echo "✓ WASM policies built successfully"
ls -lh wasm/
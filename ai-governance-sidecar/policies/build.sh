#!/bin/bash
set -e

echo "Building WASM policies..."
rustup target add wasm32-unknown-unknown 2>/dev/null || true
mkdir -p wasm

# Compile the library once
cargo build --release --target wasm32-unknown-unknown --lib

# The output is a single .wasm file - we can use it for all policies
# Or create symlinks if your Go code expects different filenames
cp target/wasm32-unknown-unknown/release/governance_policies.wasm wasm/sensitive_data.wasm
cp target/wasm32-unknown-unknown/release/governance_policies.wasm wasm/rate_limit.wasm  
cp target/wasm32-unknown-unknown/release/governance_policies.wasm wasm/passthrough.wasm

echo "✓ Done"
ls -lh wasm/
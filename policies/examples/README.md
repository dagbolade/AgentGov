# WASM Policy Examples

This directory contains production-ready WASM policies built with Open Policy Agent (OPA) for the AgentGov governance sidecar.

## Policies

### 1. `allow_all.wasm` - Development/Testing Policy
- **Purpose**: Allows all requests (for development and testing environments)
- **Source**: `allow_all.rego`
- **Entrypoint**: `example/is_allowed`
- **Input**: Any (always returns true)
- **Output**: Always `true`

### 2. `business_hours.wasm` - Business Hours Policy
- **Purpose**: Only allows requests during business hours (9am-5pm)
- **Source**: `business_hours.rego`
- **Entrypoint**: `example/is_allowed`
- **Input**: `{"hour": 14}` (integer, 0-23)
- **Output**: `true` if 9 <= hour <= 17, `false` otherwise

### 3. `require_approval.wasm` - Approval-Based Policy
- **Purpose**: Requires human approval for sensitive operations
- **Source**: `require_approval.rego`
- **Entrypoint**: `example/is_allowed`
- **Input**: `{"sensitive": true, "approved": false}`
- **Output**: `true` if not sensitive OR if sensitive and approved

## Usage

Load these WASM policies in your governance sidecar by:

1. Configuring the policy engine to load the `.wasm` files
2. Setting the entrypoint to `example/is_allowed`
3. Providing appropriate input JSON based on the policy requirements

## Testing

Each policy includes test files (`*_test.json`) demonstrating expected inputs and outputs.

## Building from Source

To rebuild the WASM files from the Rego source:

```bash
# Install OPA v0.63.0 (required for WASM compatibility)
curl -L -o opa https://openpolicyagent.org/downloads/v0.63.0/opa_linux_amd64
chmod +x opa

# Compile each policy
opa build -t wasm -e example/is_allowed allow_all.rego -o allow_all.wasm
opa build -t wasm -e example/is_allowed business_hours.rego -o business_hours.wasm
opa build -t wasm -e example/is_allowed require_approval.rego -o require_approval.wasm
```

## Integration

These policies are designed to integrate with the AgentGov policy engine. The expected integration pattern:

1. Load WASM policy into policy engine
2. For each incoming request, construct appropriate input JSON
3. Evaluate `example/is_allowed` with the input
4. Allow/deny the request based on the boolean result
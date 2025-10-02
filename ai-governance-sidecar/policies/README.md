# WASM Policy Engine

Production-ready policy modules for AI governance. Written in Rust, compiled to WebAssembly, executed by Go runtime.

## Architecture

Each policy is a standalone WASM module that receives tool call requests and returns authorization decisions. The Go backend loads these modules at runtime using Wasmer/Wasmtime and executes them in a sandboxed environment.

**Flow:**
```
AI Agent Request → Go Runtime → WASM Policy → Authorization Decision → Human Approval (if needed)
```

## Available Policies

### 1. Sensitive Data Detection (`sensitive_data.wasm`)
Flags requests containing:
- PII: SSN, credit cards, passwords, API keys
- Medical: patient records, health data
- Financial: bank accounts, routing numbers
- Destructive operations: DELETE, DROP, TRUNCATE

**Thresholds:** Requires approval on any sensitive keyword match.

### 2. Rate Limiting (`rate_limit.wasm`)
Enforces volume limits:
- Bulk operations: ≥100 records requires approval
- High volume: ≥1000 records requires approval
- Critical tables: ≥10 deletions on users/accounts/payments requires approval

**Parameters:** Reads `count`, `rows`, `records`, `size`, `limit` from request parameters.

### 3. Passthrough (`passthrough.wasm`)
Allows all requests. Used for testing and development.

## Building Policies

**Prerequisites:**
```
# Install Rust
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh

# Add WASM target
rustup target add wasm32-unknown-unknown
```

**Build:**
```
cd policies
./build.sh
```

Output: `wasm/sensitive_data.wasm`, `wasm/rate_limit.wasm`, `wasm/passthrough.wasm`

**Verify:**
```
ls -lh wasm/
# Each .wasm should be ~15-20KB
```

## Testing

```
# Run all tests
cargo test --all

# Run specific test suite
cargo test --test sensitive_data_test
cargo test --test rate_limit_test

# Run with output
cargo test -- --nocapture
```

## Policy Interface

### Input Structure
```
{
  "tool": "database",
  "action": "delete",
  "parameters": {
    "table": "users",
    "count": 150
  },
  "context": {
    "user_id": "admin-123",
    "timestamp": "2025-10-02T18:00:00Z"
  }
}
```

### Output Structure
```
{
  "allowed": false,
  "human_required": true,
  "reason": "Bulk delete of 150 records requires approval",
  "confidence": 0.95
}
```

### Response Fields
- `allowed`: Boolean. If false, request is blocked or queued.
- `human_required`: Boolean. If true, request goes to approval queue.
- `reason`: String. Explanation shown to approver.
- `confidence`: Float 0-1. Policy's certainty in decision.

## Creating New Policies

**1. Create policy file:**
```
touch policies/my_policy/src/lib.rs
```

**2. Add to `Cargo.toml`:**
```
[[bin]]
name = "my_policy"
path = "my_policy/src/lib.rs"
```

**3. Implement policy logic:**
```
use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::slice;
use std::str;

#[derive(Deserialize)]
struct PolicyInput {
    tool: String,
    action: String,
    parameters: Value,
    context: Value,
}

#[derive(Serialize)]
struct PolicyResult {
    allowed: bool,
    human_required: bool,
    reason: String,
    confidence: f64,
}

#[no_mangle]
pub extern "C" fn evaluate(ptr: *const u8, len: usize) -> *mut u8 {
    // Read input
    let input_bytes = unsafe { slice::from_raw_parts(ptr, len) };
    let input_str = str::from_utf8(input_bytes).unwrap();
    let input: PolicyInput = serde_json::from_str(input_str).unwrap();
    
    // Your policy logic here
    let result = PolicyResult {
        allowed: true,
        human_required: false,
        reason: "Policy logic goes here".to_string(),
        confidence: 1.0,
    };
    
    serialize_result(&result)
}

fn serialize_result(result: &PolicyResult) -> *mut u8 {
    let json = serde_json::to_string(result).unwrap();
    let bytes = json.into_bytes();
    let len = bytes.len();
    
    let total_len = 4 + len;
    let mut buf = Vec::with_capacity(total_len);
    buf.extend_from_slice(&(len as u32).to_le_bytes());
    buf.extend_from_slice(&bytes);
    
    let ptr = buf.as_ptr() as *mut u8;
    std::mem::forget(buf);
    ptr
}

#[no_mangle]
pub extern "C" fn alloc(size: usize) -> *mut u8 {
    let mut buf = Vec::with_capacity(size);
    let ptr = buf.as_mut_ptr();
    std::mem::forget(buf);
    ptr
}

#[no_mangle]
pub extern "C" fn dealloc(ptr: *mut u8, size: usize) {
    unsafe { let _ = Vec::from_raw_parts(ptr, size, size); }
}
```

**4. Update build script:**
```
# Add to POLICIES array in build.sh
POLICIES=("sensitive_data" "rate_limit" "passthrough" "my_policy")
```

**5. Build and test:**
```
./build.sh
cargo test --test my_policy_test
```

## Go Runtime Integration

The Go backend loads WASM files from `POLICY_DIR` environment variable (default: `/app/policies`).

**Configuration:**
```
# docker-compose.yml
environment:
  - POLICY_DIR=/app/policies
volumes:
  - ./policies/wasm:/app/policies:ro
```

**Go code loads and executes:**
```
// internal/policy/evaluator.go
func (e *Evaluator) Evaluate(ctx context.Context, req Request) (Response, error) {
    // Serializes req to JSON
    // Calls WASM evaluate() function
    // Deserializes Response from WASM
}
```

## Performance Characteristics

- **Cold start:** ~5ms per policy load
- **Execution:** ~0.1-1ms per evaluation
- **Memory:** ~2-5MB per loaded policy
- **Concurrency:** Thread-safe, policies share no state

## Production Considerations

**Security:**
- WASM sandbox prevents file system access
- No network access from policies
- Memory isolated per execution

**Scaling:**
- Policies are stateless - safe to parallelize
- Consider caching policy instances
- Monitor evaluation latency

**Debugging:**
- WASM has limited debugging support
- Add detailed logging to `reason` field
- Use `confidence` to indicate edge cases

**Updates:**
- Hot-reload supported by reloading WASM files
- No downtime required for policy updates
- Version policies using filenames: `sensitive_data_v2.wasm`

## Troubleshooting

**Build fails with "missing serde":**
```
cargo clean
cargo build --release --target wasm32-unknown-unknown
```

**WASM file not found in Docker:**
```
# Verify files exist
ls -la policies/wasm/

# Check volume mount
docker-compose exec governance-sidecar ls -la /app/policies/
```

**Policy not triggering:**
- Check Go logs for policy loading errors
- Verify WASM exports `evaluate` function
- Validate input JSON structure matches PolicyInput

**High latency:**
- Profile with `cargo bench`
- Reduce string operations
- Consider caching expensive computations

## Development Workflow

```
# 1. Edit policy
vim sensitive_data/src/lib.rs

# 2. Test
cargo test --test sensitive_data_test

# 3. Build WASM
./build.sh

# 4. Rebuild Docker
docker-compose up --build

# 5. Test end-to-end
curl -X POST http://localhost:8080/proxy \
  -H "Content-Type: application/json" \
  -d '{"tool":"db","action":"query","parameters":{"query":"SELECT password FROM users"}}'
```

## File Structure

```
policies/
├── README.md              # This file
├── Cargo.toml             # Rust workspace
├── build.sh               # Build script
├── src/
│   └── lib.rs            # Shared utilities
├── sensitive_data/
│   └── src/lib.rs        # Sensitive data policy
├── rate_limit/
│   └── src/lib.rs        # Rate limit policy
├── passthrough/
│   └── src/lib.rs        # Passthrough policy
├── tests/                 # Test suites
│   ├── sensitive_data_test.rs
│   ├── rate_limit_test.rs
│   ├── passthrough_test.rs
│   └── integration_test.rs
└── wasm/                  # Compiled outputs (gitignored)
    ├── sensitive_data.wasm
    ├── rate_limit.wasm
    └── passthrough.wasm
```

## References

- [WebAssembly Specification](https://webassembly.github.io/spec/)
- [Wasmer Runtime](https://wasmer.io/)
- [Rust WASM Book](https://rustwasm.github.io/docs/book/)
- [Project Main README](../README.md)

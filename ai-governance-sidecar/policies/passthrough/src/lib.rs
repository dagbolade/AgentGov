// Real implementation - allows all requests (for testing/development)
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
    // Read input from Go runtime
    let input_bytes = unsafe { slice::from_raw_parts(ptr, len) };
    let input_str = match str::from_utf8(input_bytes) {
        Ok(s) => s,
        Err(_) => return error_result("Invalid UTF-8 input"),
    };
    
    // Parse JSON input (validate it's valid)
    let input: PolicyInput = match serde_json::from_str(input_str) {
        Ok(i) => i,
        Err(e) => return error_result(&format!("Invalid JSON: {}", e)),
    };
    
    // Passthrough - allow everything
    let result = PolicyResult {
        allowed: true,
        human_required: false,
        reason: format!(
            "Passthrough policy: allowing {}.{} request",
            input.tool, input.action
        ),
        confidence: 1.0,
    };
    
    serialize_result(&result)
}

fn error_result(message: &str) -> *mut u8 {
    let result = PolicyResult {
        allowed: false,
        human_required: false,
        reason: message.to_string(),
        confidence: 1.0,
    };
    serialize_result(&result)
}

fn serialize_result(result: &PolicyResult) -> *mut u8 {
    let json = serde_json::to_string(result).unwrap();
    let bytes = json.into_bytes();
    let len = bytes.len();
    
    // Allocate memory for length prefix (4 bytes) + JSON data
    let total_len = 4 + len;
    let mut buf = Vec::with_capacity(total_len);
    
    // Write length as little-endian u32
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
    unsafe {
        let _ = Vec::from_raw_parts(ptr, size, size);
    }
}
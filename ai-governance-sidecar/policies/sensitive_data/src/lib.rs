// Receives actual input from Go runtime
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

const SENSITIVE_KEYWORDS: &[&str] = &[
    "ssn", "social security", "credit card", "password", "api key",
    "secret", "private key", "token", "credentials", "delete all",
    "drop table", "drop database", "rm -rf", "financial", "medical", 
    "patient", "health record", "bank account", "routing number"
];

#[no_mangle]
pub extern "C" fn evaluate(ptr: *const u8, len: usize) -> *mut u8 {
    // Read input from Go runtime
    let input_bytes = unsafe { slice::from_raw_parts(ptr, len) };
    let input_str = match str::from_utf8(input_bytes) {
        Ok(s) => s,
        Err(_) => return error_result("Invalid UTF-8 input"),
    };
    
    // Parse JSON input
    let input: PolicyInput = match serde_json::from_str(input_str) {
        Ok(i) => i,
        Err(e) => return error_result(&format!("Invalid JSON: {}", e)),
    };
    
    // Check for sensitive data
    let params_str = input.parameters.to_string().to_lowercase();
    let action_str = input.action.to_lowercase();
    let tool_str = input.tool.to_lowercase();
    
    let mut found_sensitive = Vec::new();
    for keyword in SENSITIVE_KEYWORDS {
        if params_str.contains(keyword) || action_str.contains(keyword) || tool_str.contains(keyword) {
            found_sensitive.push(*keyword);
        }
    }
    
    // Specific checks for dangerous operations
    let is_delete = action_str.contains("delete") || action_str.contains("remove") || action_str.contains("drop");
    let is_destructive = action_str.contains("truncate") || action_str.contains("destroy");
    
    let result = if !found_sensitive.is_empty() {
        PolicyResult {
            allowed: false,
            human_required: true,
            reason: format!(
                "Sensitive data detected: {}. Human approval required before proceeding.",
                found_sensitive.join(", ")
            ),
            confidence: 0.95,
        }
    } else if is_delete || is_destructive {
        // Even without keywords, destructive operations need review
        PolicyResult {
            allowed: false,
            human_required: true,
            reason: format!("Destructive operation '{}' requires human approval.", input.action),
            confidence: 0.85,
        }
    } else {
        PolicyResult {
            allowed: true,
            human_required: false,
            reason: "No sensitive data or destructive operations detected.".to_string(),
            confidence: 1.0,
        }
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
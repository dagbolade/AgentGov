// Real implementation - receives actual input from Go runtime
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

const HIGH_VOLUME_THRESHOLD: i64 = 1000;
const BULK_DELETE_THRESHOLD: i64 = 100;
const CRITICAL_DELETE_THRESHOLD: i64 = 10; // For critical tables

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
    
    // Check operation type
    let action_lower = input.action.to_lowercase();
    let is_bulk = action_lower.contains("bulk") || action_lower.contains("batch");
    let is_delete = action_lower.contains("delete") || action_lower.contains("remove");
    let is_update = action_lower.contains("update") || action_lower.contains("modify");
    
    // Extract record count from various parameter formats
    let count = extract_record_count(&input.parameters);
    let limit = input.parameters.get("limit")
        .and_then(|v| v.as_i64())
        .unwrap_or(0);
    let records_affected = count.max(limit);
    
    // Check for critical table operations
    let table = input.parameters.get("table")
        .and_then(|v| v.as_str())
        .unwrap_or("");
    let is_critical_table = is_critical_table_name(table);
    
    // Determine if approval is needed
    let result = if is_delete && is_critical_table && records_affected >= CRITICAL_DELETE_THRESHOLD {
        PolicyResult {
            allowed: false,
            human_required: true,
            reason: format!(
                "Critical: Deleting {} records from critical table '{}'. Human approval required.",
                records_affected, table
            ),
            confidence: 1.0,
        }
    } else if is_delete && records_affected >= BULK_DELETE_THRESHOLD {
        PolicyResult {
            allowed: false,
            human_required: true,
            reason: format!(
                "Bulk delete of {} records requires approval to prevent accidental data loss.",
                records_affected
            ),
            confidence: 0.95,
        }
    } else if (is_bulk || is_update) && records_affected >= HIGH_VOLUME_THRESHOLD {
        PolicyResult {
            allowed: false,
            human_required: true,
            reason: format!(
                "High-volume operation affecting {} records. Approval required to ensure intentional execution.",
                records_affected
            ),
            confidence: 0.9,
        }
    } else if records_affected > 0 {
        PolicyResult {
            allowed: true,
            human_required: false,
            reason: format!("Operation within normal limits ({} records).", records_affected),
            confidence: 1.0,
        }
    } else {
        PolicyResult {
            allowed: true,
            human_required: false,
            reason: "Operation does not specify record count, allowing.".to_string(),
            confidence: 0.8,
        }
    };
    
    serialize_result(&result)
}

fn extract_record_count(params: &Value) -> i64 {
    // Try multiple common parameter names
    params.get("count")
        .or_else(|| params.get("rows"))
        .or_else(|| params.get("records"))
        .or_else(|| params.get("size"))
        .and_then(|v| v.as_i64())
        .unwrap_or(0)
}

fn is_critical_table_name(table: &str) -> bool {
    let critical_tables = [
        "users", "accounts", "payments", "transactions", 
        "credentials", "auth", "sessions", "audit"
    ];
    
    let table_lower = table.to_lowercase();
    critical_tables.iter().any(|&t| table_lower.contains(t))
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
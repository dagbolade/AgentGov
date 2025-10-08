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
pub extern "C" fn evaluate(input_ptr: *const u8, input_len: usize, output_ptr: *mut u8, output_len: usize) -> i32 {
    // Read input from Go runtime
    let input_bytes = unsafe { slice::from_raw_parts(input_ptr, input_len) };
    let input_str = match str::from_utf8(input_bytes) {
        Ok(s) => s,
        Err(_) => {
            write_error_output(output_ptr, output_len, "Invalid UTF-8 input");
            return 1;
        }
    };
    
    // Parse JSON input (validate it's valid)
    let input: PolicyInput = match serde_json::from_str(input_str) {
        Ok(i) => i,
        Err(e) => {
            write_error_output(output_ptr, output_len, &format!("Invalid JSON: {}", e));
            return 1;
        }
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
    
    write_result_output(output_ptr, output_len, &result)
}

fn write_result_output(output_ptr: *mut u8, output_len: usize, result: &PolicyResult) -> i32 {
    let json = match serde_json::to_string(result) {
        Ok(j) => j,
        Err(e) => {
            write_error_output(output_ptr, output_len, &format!("Serialization error: {}", e));
            return 1;
        }
    };
    
    let bytes = json.as_bytes();
    let copy_len = bytes.len().min(output_len);
    
    unsafe {
        std::ptr::copy_nonoverlapping(bytes.as_ptr(), output_ptr, copy_len);
    }
    
    0 // Success
}

fn write_error_output(output_ptr: *mut u8, output_len: usize, message: &str) {
    let result = PolicyResult {
        allowed: false,
        human_required: false,
        reason: message.to_string(),
        confidence: 0.0,
    };
    
    let _ = write_result_output(output_ptr, output_len, &result);
}

#[no_mangle]
pub extern "C" fn allocate(size: usize) -> *mut u8 {
    let layout = std::alloc::Layout::from_size_align(size, 1).unwrap();
    unsafe { std::alloc::alloc(layout) }
}

#[no_mangle]
pub extern "C" fn dealloc(ptr: *mut u8, size: usize) {
    unsafe {
        let _ = Vec::from_raw_parts(ptr, size, size);
    }
}
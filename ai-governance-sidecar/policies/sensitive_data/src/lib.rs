// Shared library code for all policies
use serde::{Deserialize, Serialize};
use serde_json::Value;

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct PolicyInput {
    pub tool: String,
    pub action: String,
    pub parameters: Value,
    pub context: Value,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct PolicyResult {
    pub allowed: bool,
    pub human_required: bool,
    pub reason: String,
    pub confidence: f64,
}

impl PolicyResult {
    pub fn allow(reason: impl Into<String>) -> Self {
        Self {
            allowed: true,
            human_required: false,
            reason: reason.into(),
            confidence: 1.0,
        }
    }
    
    pub fn deny(reason: impl Into<String>) -> Self {
        Self {
            allowed: false,
            human_required: false,
            reason: reason.into(),
            confidence: 1.0,
        }
    }
    
    pub fn require_approval(reason: impl Into<String>, confidence: f64) -> Self {
        Self {
            allowed: false,
            human_required: true,
            reason: reason.into(),
            confidence,
        }
    }
}

// WASM entry point - evaluate function
// Go calls with 4 params: evaluate(inputPtr, inputLen, outputPtr, outputLen) -> i32
#[no_mangle]
pub extern "C" fn evaluate(input_ptr: *const u8, input_len: usize, output_ptr: *mut u8, output_len: usize) -> i32 {
    use std::slice;
    use std::str;
    
    // Read input from Go runtime
    let input_bytes = unsafe { slice::from_raw_parts(input_ptr, input_len) };
    let input_str = match str::from_utf8(input_bytes) {
        Ok(s) => s,
        Err(_) => {
            write_error_output(output_ptr, output_len, "Invalid UTF-8 input");
            return 1;
        }
    };
    
    // Parse JSON input
    let input: PolicyInput = match serde_json::from_str(input_str) {
        Ok(i) => i,
        Err(e) => {
            write_error_output(output_ptr, output_len, &format!("Invalid JSON: {}", e));
            return 1;
        }
    };
    
    // Default passthrough policy - allow everything
    let result = PolicyResult {
        allowed: true,
        human_required: false,
        reason: format!(
            "Policy evaluation: allowing {}.{} request",
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

// Memory management functions required for WASM
#[no_mangle]
pub extern "C" fn allocate(size: usize) -> *mut u8 {
    let layout = std::alloc::Layout::from_size_align(size, 1).unwrap();
    unsafe { std::alloc::alloc(layout) }
}

#[no_mangle]
pub extern "C" fn dealloc(ptr: *mut u8, size: usize) {
    if !ptr.is_null() && size > 0 {
        unsafe {
            let layout = std::alloc::Layout::from_size_align_unchecked(size, 1);
            std::alloc::dealloc(ptr, layout);
        }
    }
}

// Serialize result to JSON and return pointer
pub fn serialize_result(result: &PolicyResult) -> *mut u8 {
    let json = serde_json::to_string(result).unwrap_or_else(|e| {
        format!(r#"{{"allowed":false,"human_required":false,"reason":"Serialization error: {}","confidence":0.0}}"#, e)
    });
    
    let bytes = json.into_bytes();
    let ptr = bytes.as_ptr() as *mut u8;
    std::mem::forget(bytes);
    ptr
}

// Helper to check for sensitive keywords
pub fn contains_sensitive_keywords(text: &str, keywords: &[&str]) -> Vec<String> {
    let text_lower = text.to_lowercase();
    keywords
        .iter()
        .filter(|&&keyword| text_lower.contains(keyword))
        .map(|&s| s.to_string())
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_policy_result_constructors() {
        let allow = PolicyResult::allow("test");
        assert!(allow.allowed);
        assert!(!allow.human_required);
        
        let deny = PolicyResult::deny("test");
        assert!(!deny.allowed);
        assert!(!deny.human_required);
        
        let approval = PolicyResult::require_approval("test", 0.9);
        assert!(!approval.allowed);
        assert!(approval.human_required);
        assert_eq!(approval.confidence, 0.9);
    }
    
    #[test]
    fn test_sensitive_keywords() {
        let keywords = vec!["password", "secret", "api_key"];
        let text = "Please store my password in the database";
        
        let found = contains_sensitive_keywords(text, &keywords);
        assert_eq!(found, vec!["password"]);
    }
}
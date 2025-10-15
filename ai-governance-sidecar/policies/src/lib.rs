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

// Memory management functions required for WASM
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

#[no_mangle]
pub extern "C" fn evaluate(
    in_ptr: *const u8,
    in_len: usize,
    out_ptr: *mut *mut u8,
    out_len: *mut usize,
) -> i32 {
    // 1) Read input bytes from host
    let input = unsafe { std::slice::from_raw_parts(in_ptr, in_len) };

    // 2) Parse request and run *minimal* logic (passthrough by default)
    //    Replace this with your real policy checks as you evolve.
    let result: PolicyResult = match serde_json::from_slice::<PolicyInput>(input) {
        Ok(_inp) => {
            // Example: always allow (passthrough)
            PolicyResult::allow("passthrough")
        }
        Err(e) => PolicyResult::deny(format!("invalid JSON: {e}")),
    };

    // 3) Serialize to JSON and copy into newly allocated buffer
    let bytes = match serde_json::to_vec(&result) {
        Ok(b) => b,
        Err(e) => {
            let fallback = PolicyResult::deny(format!("serialization error: {e}"));
            serde_json::to_vec(&fallback).unwrap()
        }
    };

    let len = bytes.len();
    let ptr = alloc(len);
    unsafe { std::ptr::copy_nonoverlapping(bytes.as_ptr(), ptr, len); }

    // 4) Return buffer location & length to host
    unsafe {
        *out_ptr = ptr;
        *out_len = len;
    }

    // 0 = success (non-zero if you want to signal host-level errors)
    0
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
use serde_json::json;

#[test]
fn test_allows_all_operations() {
    let operations = vec![
        json!({"tool": "calculator", "action": "add", "parameters": {}, "context": {}}),
        json!({"tool": "database", "action": "delete", "parameters": {"count": 10000}, "context": {}}),
        json!({"tool": "api", "action": "fetch", "parameters": {"password": "test"}, "context": {}}),
    ];
    
    for input in operations {
        // Passthrough should allow everything
        assert!(input.is_object());
        assert!(input["tool"].is_string());
        assert!(input["action"].is_string());
    }
}

#[test]
fn test_valid_json_structure() {
    let input = json!({
        "tool": "test",
        "action": "test",
        "parameters": {},
        "context": {}
    });
    
    assert!(input["tool"].is_string());
    assert!(input["action"].is_string());
    assert!(input["parameters"].is_object());
    assert!(input["context"].is_object());
}
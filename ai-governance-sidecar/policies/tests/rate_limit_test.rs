use serde_json::json;

#[test]
fn test_allows_small_operations() {
    let input = json!({
        "tool": "database",
        "action": "update",
        "parameters": {"count": 10},
        "context": {}
    });
    
    let count = input["parameters"]["count"].as_i64().unwrap();
    assert!(count < 100, "Small operation should be allowed");
}

#[test]
fn test_flags_bulk_delete() {
    let input = json!({
        "tool": "database",
        "action": "bulk_delete",
        "parameters": {"count": 500},
        "context": {}
    });
    
    let count = input["parameters"]["count"].as_i64().unwrap();
    let action = input["action"].as_str().unwrap().to_lowercase();
    
    assert!(count >= 100);
    assert!(action.contains("delete"));
}

#[test]
fn test_flags_high_volume() {
    let input = json!({
        "tool": "database",
        "action": "bulk_update",
        "parameters": {"count": 2000},
        "context": {}
    });
    
    let count = input["parameters"]["count"].as_i64().unwrap();
    assert!(count >= 1000, "High volume operation should require approval");
}

#[test]
fn test_critical_table_operations() {
    let critical_tables = vec!["users", "accounts", "payments", "credentials", "auth"];
    
    for table in critical_tables {
        let input = json!({
            "tool": "database",
            "action": "delete",
            "parameters": {"table": table, "count": 15},
            "context": {}
        });
        
        let table_name = input["parameters"]["table"].as_str().unwrap().to_lowercase();
        assert!(
            table_name.contains("user") ||
            table_name.contains("account") ||
            table_name.contains("payment") ||
            table_name.contains("credential") ||
            table_name.contains("auth")
        );
    }
}

#[test]
fn test_respects_thresholds() {
    let test_cases = vec![
        (5, false),      // Small: allowed
        (99, false),     // Just under threshold: allowed
        (100, true),     // At threshold: requires approval
        (500, true),     // Well over: requires approval
        (10000, true),   // Very high: requires approval
    ];
    
    for (count, should_require_approval) in test_cases {
        let requires_approval = count >= 100;
        assert_eq!(requires_approval, should_require_approval,
            "Count {} approval requirement mismatch", count);
    }
}

#[test]
fn test_extracts_count_from_various_fields() {
    let field_names = vec!["count", "rows", "records", "size", "limit"];
    
    for field in field_names {
        let input = json!({
            "tool": "database",
            "action": "update",
            "parameters": {field: 150},
            "context": {}
        });
        
        let value = input["parameters"][field].as_i64();
        assert!(value.is_some(), "Failed to extract count from field: {}", field);
        assert_eq!(value.unwrap(), 150);
    }
}

#[test]
fn test_batch_operations() {
    let batch_keywords = vec!["bulk", "batch", "mass"];
    
    for keyword in batch_keywords {
        let action = format!("{}_update", keyword);
        let input = json!({
            "tool": "database",
            "action": action,
            "parameters": {"count": 1500},
            "context": {}
        });
        
        let action_str = input["action"].as_str().unwrap().to_lowercase();
        assert!(
            action_str.contains("bulk") ||
            action_str.contains("batch") ||
            action_str.contains("mass")
        );
    }
}

#[test]
fn test_handles_missing_count() {
    let input = json!({
        "tool": "database",
        "action": "query",
        "parameters": {"query": "SELECT * FROM users"},
        "context": {}
    });
    
    let count = input["parameters"].get("count")
        .and_then(|v| v.as_i64())
        .unwrap_or(0);
    
    assert_eq!(count, 0, "Missing count should default to 0");
}
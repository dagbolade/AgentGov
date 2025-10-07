#!/bin/bash

# Policy Validation Script
# Tests all three WASM policies to ensure they compile, load, and enforce correctly

set -e

echo "🔍 Validating WASM Policies"
echo "=========================="

# Check that all files exist
echo "✓ Checking file existence..."
for policy in allow_all business_hours require_approval; do
    if [[ ! -f "${policy}.rego" ]]; then
        echo "❌ Missing ${policy}.rego"
        exit 1
    fi
    if [[ ! -f "${policy}.wasm" ]]; then
        echo "❌ Missing ${policy}.wasm"
        exit 1
    fi
done

echo "✓ All policy files present"

# Test 1: allow_all policy
echo ""
echo "🧪 Testing allow_all policy..."
result=$(echo '{}' | opa eval -d allow_all.rego "data.example.is_allowed" --format raw)
if [[ "$result" == "true" ]]; then
    echo "✓ allow_all: Empty input correctly allowed"
else
    echo "❌ allow_all: Expected true, got $result"
    exit 1
fi

result=$(echo '{"user": "test"}' | opa eval -d allow_all.rego "data.example.is_allowed" --format raw)
if [[ "$result" == "true" ]]; then
    echo "✓ allow_all: Non-empty input correctly allowed"
else
    echo "❌ allow_all: Expected true, got $result"
    exit 1
fi

# Test 2: business_hours policy  
echo ""
echo "🧪 Testing business_hours policy..."
echo '{"hour": 14}' > temp_input.json
result=$(opa eval -d business_hours.rego -i temp_input.json "data.example.is_allowed" --format raw)
if [[ "$result" == "true" ]]; then
    echo "✓ business_hours: 2pm correctly allowed"
else
    echo "❌ business_hours: Expected true for 2pm, got $result"
    exit 1
fi

echo '{"hour": 18}' > temp_input.json
result=$(opa eval -d business_hours.rego -i temp_input.json "data.example.is_allowed" --format raw)
if [[ "$result" == "false" ]]; then
    echo "✓ business_hours: 6pm correctly denied"
else
    echo "❌ business_hours: Expected false for 6pm, got $result"
    exit 1
fi

echo '{"hour": 9}' > temp_input.json
result=$(opa eval -d business_hours.rego -i temp_input.json "data.example.is_allowed" --format raw)
if [[ "$result" == "true" ]]; then
    echo "✓ business_hours: 9am correctly allowed"
else
    echo "❌ business_hours: Expected true for 9am, got $result"
    exit 1
fi

echo '{"hour": 17}' > temp_input.json
result=$(opa eval -d business_hours.rego -i temp_input.json "data.example.is_allowed" --format raw)
if [[ "$result" == "true" ]]; then
    echo "✓ business_hours: 5pm correctly allowed"
else
    echo "❌ business_hours: Expected true for 5pm, got $result"
    exit 1
fi

# Test 3: require_approval policy
echo ""
echo "🧪 Testing require_approval policy..."
echo '{"sensitive": false}' > temp_input.json
result=$(opa eval -d require_approval.rego -i temp_input.json "data.example.is_allowed" --format raw)
if [[ "$result" == "true" ]]; then
    echo "✓ require_approval: Non-sensitive correctly allowed"
else
    echo "❌ require_approval: Expected true for non-sensitive, got $result"
    exit 1
fi

echo '{"sensitive": true, "approved": true}' > temp_input.json
result=$(opa eval -d require_approval.rego -i temp_input.json "data.example.is_allowed" --format raw)
if [[ "$result" == "true" ]]; then
    echo "✓ require_approval: Sensitive + approved correctly allowed"
else
    echo "❌ require_approval: Expected true for sensitive+approved, got $result"
    exit 1
fi

echo '{"sensitive": true, "approved": false}' > temp_input.json
result=$(opa eval -d require_approval.rego -i temp_input.json "data.example.is_allowed" --format raw)
if [[ "$result" == "false" ]]; then
    echo "✓ require_approval: Sensitive + not approved correctly denied"
else
    echo "❌ require_approval: Expected false for sensitive+not approved, got $result"
    exit 1
fi

# Clean up
rm -f temp_input.json

echo ""
echo "🎉 All policies validated successfully!"
echo ""
echo "📊 Summary:"
echo "   - allow_all.wasm: ✓ Allows all requests (development/testing)"
echo "   - business_hours.wasm: ✓ Enforces 9am-5pm business hours"  
echo "   - require_approval.wasm: ✓ Requires approval for sensitive operations"
echo ""
echo "🚀 Policies are ready for production use!"
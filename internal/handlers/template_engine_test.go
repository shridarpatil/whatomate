package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- processTemplate ---

func TestProcessTemplate_EmptyTemplate(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", processTemplate("", nil))
}

func TestProcessTemplate_NoPlaceholders(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "Hello, World!", processTemplate("Hello, World!", nil))
}

func TestProcessTemplate_VariablesOnly(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{"name": "Alice", "age": 30}
	result := processTemplate("Hello {{name}}, age {{age}}", data)
	assert.Equal(t, "Hello Alice, age 30", result)
}

func TestProcessTemplate_LoopsOnly(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{
		"items": []interface{}{"a", "b", "c"},
	}
	result := processTemplate("{{for item in items}}[{{item}}]{{endfor}}", data)
	assert.Equal(t, "[a][b][c]", result)
}

func TestProcessTemplate_ConditionalsOnly(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{"show": true}
	result := processTemplate("{{if show}}visible{{endif}}", data)
	assert.Equal(t, "visible", result)
}

func TestProcessTemplate_Mixed(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{
		"name":  "Alice",
		"show":  true,
		"items": []interface{}{"x", "y"},
	}
	result := processTemplate("Hi {{name}}! {{if show}}Items: {{for item in items}}{{item}} {{endfor}}{{endif}}", data)
	assert.Equal(t, "Hi Alice! Items: x y ", result)
}

func TestProcessTemplate_NilData(t *testing.T) {
	t.Parallel()
	result := processTemplate("Hello {{name}}", nil)
	assert.Equal(t, "Hello ", result)
}

func TestProcessTemplate_NestedLoopsWithConditionals(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{
		"users": []interface{}{
			map[string]interface{}{"name": "Alice", "active": true},
			map[string]interface{}{"name": "Bob", "active": false},
		},
	}
	result := processTemplate("{{for user in users}}{{if user.active}}*{{user.name}}*{{else}}({{user.name}}){{endif}} {{endfor}}", data)
	assert.Equal(t, "*Alice* (Bob) ", result)
}

// --- processForLoops ---

func TestProcessForLoops_EmptyArray(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{"items": []interface{}{}}
	result := processForLoops("{{for item in items}}{{item}}{{endfor}}", data)
	assert.Equal(t, "", result)
}

func TestProcessForLoops_SingleItem(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{"items": []interface{}{"hello"}}
	result := processForLoops("{{for item in items}}[{{item}}]{{endfor}}", data)
	// After processForLoops, variables are processed within the loop body
	assert.Equal(t, "[hello]", result)
}

func TestProcessForLoops_MultipleItems(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{
		"colors": []interface{}{"red", "green", "blue"},
	}
	result := processForLoops("{{for color in colors}}{{color}},{{endfor}}", data)
	assert.Equal(t, "red,green,blue,", result)
}

func TestProcessForLoops_NestedObjectAccess(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{
		"users": []interface{}{
			map[string]interface{}{"name": "Alice", "email": "alice@test.com"},
			map[string]interface{}{"name": "Bob", "email": "bob@test.com"},
		},
	}
	result := processForLoops("{{for user in users}}{{user.name}}:{{user.email}} {{endfor}}", data)
	assert.Equal(t, "Alice:alice@test.com Bob:bob@test.com ", result)
}

func TestProcessForLoops_MissingArray(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{}
	result := processForLoops("before{{for item in missing}}{{item}}{{endfor}}after", data)
	assert.Equal(t, "beforeafter", result)
}

func TestProcessForLoops_IndexVariable(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{
		"items": []interface{}{"a", "b", "c"},
	}
	result := processForLoops("{{for item in items}}{{item_index}}:{{item}} {{endfor}}", data)
	assert.Equal(t, "0:a 1:b 2:c ", result)
}

func TestProcessForLoops_MapSlice(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{
		"products": []map[string]interface{}{
			{"name": "Widget", "price": 9.99},
			{"name": "Gadget", "price": 19.99},
		},
	}
	result := processForLoops("{{for p in products}}{{p.name}} {{endfor}}", data)
	assert.Equal(t, "Widget Gadget ", result)
}

func TestProcessForLoops_NonArrayValue(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{"items": "not-an-array"}
	result := processForLoops("before{{for item in items}}{{item}}{{endfor}}after", data)
	assert.Equal(t, "beforeafter", result)
}

func TestProcessForLoops_NestedPath(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{
		"data": map[string]interface{}{
			"items": []interface{}{"x", "y"},
		},
	}
	result := processForLoops("{{for item in data.items}}{{item}}{{endfor}}", data)
	assert.Equal(t, "xy", result)
}

// --- processConditionals ---

func TestProcessConditionals_TruthyCondition(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{"visible": true}
	result := processConditionals("{{if visible}}shown{{endif}}", data)
	assert.Equal(t, "shown", result)
}

func TestProcessConditionals_FalsyCondition(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{"visible": false}
	result := processConditionals("{{if visible}}shown{{endif}}", data)
	assert.Equal(t, "", result)
}

func TestProcessConditionals_ElseBranch(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{"logged_in": false}
	result := processConditionals("{{if logged_in}}Welcome{{else}}Please login{{endif}}", data)
	assert.Equal(t, "Please login", result)
}

func TestProcessConditionals_ElseBranchTrue(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{"logged_in": true}
	result := processConditionals("{{if logged_in}}Welcome{{else}}Please login{{endif}}", data)
	assert.Equal(t, "Welcome", result)
}

func TestProcessConditionals_NumericGreaterThan(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{"score": 85}
	result := processConditionals("{{if score > 80}}pass{{else}}fail{{endif}}", data)
	assert.Equal(t, "pass", result)
}

func TestProcessConditionals_NumericLessThan(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{"score": 50}
	result := processConditionals("{{if score < 60}}fail{{else}}pass{{endif}}", data)
	assert.Equal(t, "fail", result)
}

func TestProcessConditionals_StringEquality(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{"status": "active"}
	result := processConditionals("{{if status == 'active'}}online{{else}}offline{{endif}}", data)
	assert.Equal(t, "online", result)
}

func TestProcessConditionals_StringInequality(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{"status": "inactive"}
	result := processConditionals("{{if status != 'active'}}not active{{else}}active{{endif}}", data)
	assert.Equal(t, "not active", result)
}

func TestProcessConditionals_MissingVariable(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{}
	result := processConditionals("{{if missing}}yes{{else}}no{{endif}}", data)
	assert.Equal(t, "no", result)
}

func TestProcessConditionals_NestedPath(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{
		"user": map[string]interface{}{"active": true},
	}
	result := processConditionals("{{if user.active}}active{{endif}}", data)
	assert.Equal(t, "active", result)
}

func TestProcessConditionals_GreaterThanOrEqual(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{"count": 10}
	assert.Equal(t, "yes", processConditionals("{{if count >= 10}}yes{{else}}no{{endif}}", data))
	data["count"] = 9
	assert.Equal(t, "no", processConditionals("{{if count >= 10}}yes{{else}}no{{endif}}", data))
}

func TestProcessConditionals_LessThanOrEqual(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{"count": 10}
	assert.Equal(t, "yes", processConditionals("{{if count <= 10}}yes{{else}}no{{endif}}", data))
	data["count"] = 11
	assert.Equal(t, "no", processConditionals("{{if count <= 10}}yes{{else}}no{{endif}}", data))
}

// --- processVariables ---

func TestProcessVariables_Simple(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{"name": "Alice"}
	assert.Equal(t, "Hello Alice", processVariables("Hello {{name}}", data))
}

func TestProcessVariables_NestedPath(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{
		"user": map[string]interface{}{
			"profile": map[string]interface{}{"name": "Alice"},
		},
	}
	assert.Equal(t, "Hello Alice", processVariables("Hello {{user.profile.name}}", data))
}

func TestProcessVariables_ArrayIndex(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{
		"items": []interface{}{"first", "second", "third"},
	}
	assert.Equal(t, "first", processVariables("{{items[0]}}", data))
}

func TestProcessVariables_MissingVariable(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{}
	assert.Equal(t, "Hello ", processVariables("Hello {{name}}", data))
}

func TestProcessVariables_MultipleVariables(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{"first": "John", "last": "Doe"}
	assert.Equal(t, "John Doe", processVariables("{{first}} {{last}}", data))
}

func TestProcessVariables_NoPlaceholders(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "plain text", processVariables("plain text", map[string]interface{}{}))
}

// --- getNestedValue ---

func TestGetNestedValue_SimpleKey(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{"name": "Alice"}
	assert.Equal(t, "Alice", getNestedValue(data, "name"))
}

func TestGetNestedValue_NestedKey(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{
		"user": map[string]interface{}{
			"address": map[string]interface{}{"city": "NYC"},
		},
	}
	assert.Equal(t, "NYC", getNestedValue(data, "user.address.city"))
}

func TestGetNestedValue_ArrayIndex(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{
		"items": []interface{}{"a", "b", "c"},
	}
	assert.Equal(t, "b", getNestedValue(data, "items[1]"))
}

func TestGetNestedValue_ArrayOutOfBounds(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{
		"items": []interface{}{"a"},
	}
	assert.Nil(t, getNestedValue(data, "items[5]"))
}

func TestGetNestedValue_MissingKey(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{"name": "Alice"}
	assert.Nil(t, getNestedValue(data, "missing"))
}

func TestGetNestedValue_NilData(t *testing.T) {
	t.Parallel()
	assert.Nil(t, getNestedValue(nil, "name"))
}

func TestGetNestedValue_EmptyPath(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{"name": "Alice"}
	assert.Nil(t, getNestedValue(data, ""))
}

func TestGetNestedValue_DeepNestedWithArray(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{
		"data": map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{"name": "first"},
				map[string]interface{}{"name": "second"},
			},
		},
	}
	assert.Equal(t, "second", getNestedValue(data, "data.items[1].name"))
}

func TestGetNestedValue_MapSliceIndex(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{
		"records": []map[string]interface{}{
			{"id": 1},
			{"id": 2},
		},
	}
	result := getNestedValue(data, "records[0]")
	m, ok := result.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, 1, m["id"])
}

// --- splitPath ---

func TestSplitPath_Simple(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []string{"name"}, splitPath("name"))
}

func TestSplitPath_DotNotation(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []string{"user", "profile", "name"}, splitPath("user.profile.name"))
}

func TestSplitPath_BracketNotation(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []string{"items[0]"}, splitPath("items[0]"))
}

func TestSplitPath_MixedNotation(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []string{"data", "items[2]", "value"}, splitPath("data.items[2].value"))
}

func TestSplitPath_Empty(t *testing.T) {
	t.Parallel()
	assert.Empty(t, splitPath(""))
}

// --- evaluateCondition ---

func TestEvaluateCondition_TruthyVariable(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{"active": true}
	assert.True(t, evaluateCondition("active", data))
}

func TestEvaluateCondition_FalsyVariable(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{"active": false}
	assert.False(t, evaluateCondition("active", data))
}

func TestEvaluateCondition_MissingVariable(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{}
	assert.False(t, evaluateCondition("missing", data))
}

func TestEvaluateCondition_EqualOperator(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{"status": "active"}
	assert.True(t, evaluateCondition("status == 'active'", data))
	assert.False(t, evaluateCondition("status == 'inactive'", data))
}

func TestEvaluateCondition_NotEqualOperator(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{"status": "active"}
	assert.True(t, evaluateCondition("status != 'inactive'", data))
	assert.False(t, evaluateCondition("status != 'active'", data))
}

func TestEvaluateCondition_GreaterThan(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{"score": 85}
	assert.True(t, evaluateCondition("score > 80", data))
	assert.False(t, evaluateCondition("score > 90", data))
}

func TestEvaluateCondition_LessThan(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{"score": 50}
	assert.True(t, evaluateCondition("score < 60", data))
	assert.False(t, evaluateCondition("score < 40", data))
}

func TestEvaluateCondition_GreaterThanOrEqual(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{"score": 80}
	assert.True(t, evaluateCondition("score >= 80", data))
	assert.True(t, evaluateCondition("score >= 79", data))
	assert.False(t, evaluateCondition("score >= 81", data))
}

func TestEvaluateCondition_LessThanOrEqual(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{"score": 80}
	assert.True(t, evaluateCondition("score <= 80", data))
	assert.True(t, evaluateCondition("score <= 81", data))
	assert.False(t, evaluateCondition("score <= 79", data))
}

func TestEvaluateCondition_DoubleQuotes(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{"name": "Alice"}
	assert.True(t, evaluateCondition(`name == "Alice"`, data))
}

func TestEvaluateCondition_NestedPath(t *testing.T) {
	t.Parallel()
	data := map[string]interface{}{
		"user": map[string]interface{}{"role": "admin"},
	}
	assert.True(t, evaluateCondition("user.role == 'admin'", data))
}

func TestEvaluateCondition_EmptyCondition(t *testing.T) {
	t.Parallel()
	assert.False(t, evaluateCondition("", map[string]interface{}{}))
}

// --- isTruthy ---

func TestIsTruthy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value interface{}
		want  bool
	}{
		{"nil", nil, false},
		{"empty string", "", false},
		{"false string", "false", false},
		{"zero string", "0", false},
		{"false bool", false, false},
		{"zero int", int(0), false},
		{"zero int64", int64(0), false},
		{"zero float64", float64(0), false},
		{"empty slice", []interface{}{}, false},
		{"empty map slice", []map[string]interface{}{}, false},
		{"empty map", map[string]interface{}{}, false},
		{"non-empty string", "hello", true},
		{"true bool", true, true},
		{"non-zero int", 42, true},
		{"non-zero int64", int64(42), true},
		{"non-zero float64", 3.14, true},
		{"non-empty slice", []interface{}{"a"}, true},
		{"non-empty map slice", []map[string]interface{}{{"k": "v"}}, true},
		{"non-empty map", map[string]interface{}{"k": "v"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, isTruthy(tt.value))
		})
	}
}

// --- compareEqual ---

func TestCompareEqual(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   interface{}
		compare string
		want    bool
	}{
		{"string match", "hello", "hello", true},
		{"string mismatch", "hello", "world", false},
		{"int match", 42, "42", true},
		{"int mismatch", 42, "43", false},
		{"int64 match", int64(100), "100", true},
		{"float64 whole number", float64(5), "5", true},
		{"float64 decimal", 3.14, "3.14", true},
		{"bool true", true, "true", true},
		{"bool false", false, "false", true},
		{"nil vs empty", nil, "", true},
		{"nil vs null", nil, "null", true},
		{"nil vs nil string", nil, "nil", true},
		{"nil vs non-empty", nil, "hello", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, compareEqual(tt.value, tt.compare))
		})
	}
}

// --- compareNumeric ---

func TestCompareNumeric(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   interface{}
		compare string
		want    int
	}{
		{"int less", 5, "10", -1},
		{"int equal", 10, "10", 0},
		{"int greater", 15, "10", 1},
		{"int64 less", int64(5), "10", -1},
		{"int64 equal", int64(10), "10", 0},
		{"float64 less", 5.5, "10.0", -1},
		{"float64 equal", 10.0, "10", 0},
		{"float64 greater", 15.5, "10.0", 1},
		{"string number", "20", "10", 1},
		{"invalid string value", "abc", "10", 0},
		{"invalid compare value", 10, "abc", 0},
		{"non-numeric type", true, "10", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, compareNumeric(tt.value, tt.compare))
		})
	}
}

// --- formatValue ---

func TestFormatValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value interface{}
		want  string
	}{
		{"nil", nil, ""},
		{"string", "hello", "hello"},
		{"empty string", "", ""},
		{"int", 42, "42"},
		{"int64", int64(100), "100"},
		{"float64 whole", float64(5), "5"},
		{"float64 decimal", 3.14, "3.14"},
		{"bool true", true, "true"},
		{"bool false", false, "false"},
		{"slice", []interface{}{"a", "b"}, "[a b]"},
		{"map", map[string]interface{}{"k": "v"}, "map[k:v]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, formatValue(tt.value))
		})
	}
}

// --- copyMap ---

func TestCopyMap_ShallowCopy(t *testing.T) {
	t.Parallel()

	original := map[string]interface{}{"a": 1, "b": "two", "c": true}
	copied := copyMap(original)

	assert.Equal(t, original, copied)

	// Modify copy should not affect original
	copied["d"] = "new"
	assert.Nil(t, original["d"])
}

func TestCopyMap_Empty(t *testing.T) {
	t.Parallel()

	original := map[string]interface{}{}
	copied := copyMap(original)
	assert.Equal(t, 0, len(copied))
}

func TestCopyMap_NilSafe(t *testing.T) {
	t.Parallel()

	// copyMap with nil would panic (range nil map is ok but make(nil) not)
	// The implementation handles nil implicitly via make with len(nil) == 0
	copied := copyMap(nil)
	assert.NotNil(t, copied)
	assert.Equal(t, 0, len(copied))
}

// --- extractResponseMapping ---

func TestExtractResponseMapping_SimpleKey(t *testing.T) {
	t.Parallel()

	response := map[string]interface{}{"name": "Alice", "age": 30}
	mapping := map[string]string{
		"user_name": "name",
		"user_age":  "age",
	}
	result := extractResponseMapping(response, mapping)
	assert.Equal(t, "Alice", result["user_name"])
	assert.Equal(t, 30, result["user_age"])
}

func TestExtractResponseMapping_NestedPath(t *testing.T) {
	t.Parallel()

	response := map[string]interface{}{
		"data": map[string]interface{}{
			"user": map[string]interface{}{"id": "u123"},
		},
	}
	mapping := map[string]string{"uid": "data.user.id"}
	result := extractResponseMapping(response, mapping)
	assert.Equal(t, "u123", result["uid"])
}

func TestExtractResponseMapping_MissingKey(t *testing.T) {
	t.Parallel()

	response := map[string]interface{}{"name": "Alice"}
	mapping := map[string]string{"email": "email"}
	result := extractResponseMapping(response, mapping)
	assert.Nil(t, result["email"])
}

func TestExtractResponseMapping_EmptyMapping(t *testing.T) {
	t.Parallel()

	response := map[string]interface{}{"name": "Alice"}
	result := extractResponseMapping(response, map[string]string{})
	assert.Equal(t, 0, len(result))
}

func TestExtractResponseMapping_PartialMatch(t *testing.T) {
	t.Parallel()

	response := map[string]interface{}{"name": "Alice", "age": 30}
	mapping := map[string]string{
		"user_name": "name",
		"email":     "email", // missing
	}
	result := extractResponseMapping(response, mapping)
	assert.Equal(t, "Alice", result["user_name"])
	_, hasEmail := result["email"]
	assert.False(t, hasEmail)
}

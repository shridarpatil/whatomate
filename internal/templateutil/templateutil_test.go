package templateutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtParamNames_PositionalParams(t *testing.T) {
	content := "Hello {{1}}, your order {{2}} is ready!"
	result := ExtParamNames(content)
	assert.Equal(t, []string{"1", "2"}, result)
}

func TestExtParamNames_NamedParams(t *testing.T) {
	content := "Hello {{name}}, your order {{order_id}} is ready!"
	result := ExtParamNames(content)
	assert.Equal(t, []string{"name", "order_id"}, result)
}

func TestExtParamNames_MixedParams(t *testing.T) {
	content := "Hello {{1}}, your order {{order_id}} is ready! Amount: {{3}}"
	result := ExtParamNames(content)
	assert.Equal(t, []string{"1", "order_id", "3"}, result)
}

func TestExtParamNames_NoParams(t *testing.T) {
	content := "Hello, your order is ready!"
	result := ExtParamNames(content)
	assert.Nil(t, result)
}

func TestExtParamNames_DuplicateParams(t *testing.T) {
	content := "Hello {{name}}, {{name}} your order {{order_id}} is ready!"
	result := ExtParamNames(content)
	assert.Equal(t, []string{"name", "order_id"}, result)
}

func TestExtParamNames_UnderscoreParams(t *testing.T) {
	content := "Hello {{customer_name}}, order {{order_number}} total {{total_amount}}"
	result := ExtParamNames(content)
	assert.Equal(t, []string{"customer_name", "order_number", "total_amount"}, result)
}

func TestResolveParamsFromMap_NamedMatch(t *testing.T) {
	paramNames := []string{"name", "order_id"}
	params := map[string]string{
		"name":     "John",
		"order_id": "ORD-123",
	}
	result := ResolveParamsFromMap(paramNames, params)
	assert.Equal(t, []string{"John", "ORD-123"}, result)
}

func TestResolveParamsFromMap_PositionalMatch(t *testing.T) {
	paramNames := []string{"1", "2"}
	params := map[string]string{
		"1": "John",
		"2": "ORD-123",
	}
	result := ResolveParamsFromMap(paramNames, params)
	assert.Equal(t, []string{"John", "ORD-123"}, result)
}

func TestResolveParamsFromMap_FallbackToPositional(t *testing.T) {
	paramNames := []string{"name", "order_id"}
	params := map[string]string{
		"1": "John",
		"2": "ORD-123",
	}
	result := ResolveParamsFromMap(paramNames, params)
	assert.Equal(t, []string{"John", "ORD-123"}, result)
}

func TestResolveParamsFromMap_MissingParams(t *testing.T) {
	paramNames := []string{"name", "order_id"}
	params := map[string]string{
		"name": "John",
	}
	result := ResolveParamsFromMap(paramNames, params)
	assert.Equal(t, []string{"John", ""}, result)
}

func TestResolveParamsFromMap_EmptyInputs(t *testing.T) {
	result1 := ResolveParamsFromMap([]string{}, map[string]string{"a": "b"})
	assert.Nil(t, result1)

	result2 := ResolveParamsFromMap([]string{"a"}, map[string]string{})
	assert.Nil(t, result2)

	result3 := ResolveParamsFromMap([]string{}, map[string]string{})
	assert.Nil(t, result3)
}

func TestResolveParams(t *testing.T) {
	bodyContent := "Hello {{name}}, order {{order_id}}"
	params := map[string]interface{}{
		"name":     "John",
		"order_id": "ORD-123",
	}
	result := ResolveParams(bodyContent, params)
	assert.Equal(t, []string{"John", "ORD-123"}, result)
}

func TestResolveParams_Positional(t *testing.T) {
	bodyContent := "Hello {{name}}, order {{order_id}}"
	params := map[string]interface{}{
		"1": "John",
		"2": "ORD-123",
	}
	result := ResolveParams(bodyContent, params)
	assert.Equal(t, []string{"John", "ORD-123"}, result)
}

func TestResolveParams_Empty(t *testing.T) {
	result := ResolveParams("Hello {{name}}", map[string]interface{}{})
	assert.Nil(t, result)

	result = ResolveParams("Hello world", map[string]interface{}{"a": "b"})
	assert.Nil(t, result)
}

func TestReplaceWithStringParams(t *testing.T) {
	content := "Hello {{name}}, your order {{order_id}} is ready!"
	params := map[string]string{
		"name":     "John",
		"order_id": "ORD-123",
	}
	result := ReplaceWithStringParams(content, params)
	assert.Equal(t, "Hello John, your order ORD-123 is ready!", result)
}

func TestReplaceWithStringParams_Empty(t *testing.T) {
	assert.Equal(t, "", ReplaceWithStringParams("", map[string]string{"a": "b"}))
	assert.Equal(t, "hello", ReplaceWithStringParams("hello", map[string]string{}))
}

func TestReplaceWithJSONBParams(t *testing.T) {
	bodyContent := "Hello {{name}}, order {{order_id}}"
	content := "Hello {{name}}, order {{order_id}}"
	params := map[string]interface{}{
		"name":     "John",
		"order_id": "ORD-123",
	}
	result := ReplaceWithJSONBParams(bodyContent, content, params)
	assert.Equal(t, "Hello John, order ORD-123", result)
}

func TestReplaceWithJSONBParams_Empty(t *testing.T) {
	result := ReplaceWithJSONBParams("Hello {{name}}", "Hello {{name}}", map[string]interface{}{})
	assert.Equal(t, "Hello {{name}}", result)
}

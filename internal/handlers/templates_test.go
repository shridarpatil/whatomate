package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractParameterNames_PositionalParams(t *testing.T) {
	content := "Hello {{1}}, your order {{2}} is ready!"
	result := extractParameterNames(content)
	assert.Equal(t, []string{"1", "2"}, result)
}

func TestExtractParameterNames_NamedParams(t *testing.T) {
	content := "Hello {{name}}, your order {{order_id}} is ready!"
	result := extractParameterNames(content)
	assert.Equal(t, []string{"name", "order_id"}, result)
}

func TestExtractParameterNames_MixedParams(t *testing.T) {
	content := "Hello {{1}}, your order {{order_id}} is ready! Amount: {{3}}"
	result := extractParameterNames(content)
	assert.Equal(t, []string{"1", "order_id", "3"}, result)
}

func TestExtractParameterNames_NoParams(t *testing.T) {
	content := "Hello, your order is ready!"
	result := extractParameterNames(content)
	assert.Nil(t, result)
}

func TestExtractParameterNames_DuplicateParams(t *testing.T) {
	content := "Hello {{name}}, {{name}} your order {{order_id}} is ready!"
	result := extractParameterNames(content)
	// Should only return unique names in order of first occurrence
	assert.Equal(t, []string{"name", "order_id"}, result)
}

func TestExtractParameterNames_UnderscoreParams(t *testing.T) {
	content := "Hello {{customer_name}}, order {{order_number}} total {{total_amount}}"
	result := extractParameterNames(content)
	assert.Equal(t, []string{"customer_name", "order_number", "total_amount"}, result)
}

package handlers

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Template syntax patterns
var (
	// {{for item in items}}...{{endfor}}
	forLoopPattern = regexp.MustCompile(`\{\{for\s+(\w+)\s+in\s+(\w+(?:\.\w+)*)\}\}([\s\S]*?)\{\{endfor\}\}`)

	// {{if condition}}...{{else}}...{{endif}} or {{if condition}}...{{endif}}
	ifElsePattern = regexp.MustCompile(`\{\{if\s+([^}]+)\}\}([\s\S]*?)\{\{endif\}\}`)

	// {{variable}} or {{object.nested.path}} or {{array[0].field}}
	variablePattern = regexp.MustCompile(`\{\{([a-zA-Z_][a-zA-Z0-9_]*(?:\.[a-zA-Z_][a-zA-Z0-9_]*|\[\d+\])*)\}\}`)

	// Condition parsing: variable, variable == 'value', variable > 100, etc.
	conditionPattern = regexp.MustCompile(`^(\w+(?:\.\w+)*)\s*(==|!=|>|<|>=|<=)?\s*(.*)$`)
)

const maxLoopIterations = 50

// processTemplate processes a template string with variables, conditionals, and loops
func processTemplate(template string, data map[string]interface{}) string {
	if data == nil {
		data = make(map[string]interface{})
	}

	result := template

	// 1. Process for loops first (they may contain if blocks and variables)
	result = processForLoops(result, data)

	// 2. Process if/else conditionals
	result = processConditionals(result, data)

	// 3. Process remaining variable replacements
	result = processVariables(result, data)

	return result
}

// processForLoops handles {{for item in items}}...{{endfor}} blocks
func processForLoops(template string, data map[string]interface{}) string {
	result := template

	for {
		match := forLoopPattern.FindStringSubmatchIndex(result)
		if match == nil {
			break
		}

		// Extract loop parts
		fullMatch := result[match[0]:match[1]]
		itemVar := result[match[2]:match[3]]
		arrayPath := result[match[4]:match[5]]
		loopBody := result[match[6]:match[7]]

		// Get the array from data
		arrayValue := getNestedValue(data, arrayPath)

		var output strings.Builder

		// Process each item in the array
		switch arr := arrayValue.(type) {
		case []interface{}:
			iterations := len(arr)
			if iterations > maxLoopIterations {
				iterations = maxLoopIterations
			}
			for i := 0; i < iterations; i++ {
				// Create a new data context with the loop variable
				loopData := copyMap(data)
				loopData[itemVar] = arr[i]
				loopData[itemVar+"_index"] = i

				// Process the loop body with the loop context
				processedBody := processConditionals(loopBody, loopData)
				processedBody = processVariables(processedBody, loopData)
				output.WriteString(processedBody)
			}

		case []map[string]interface{}:
			iterations := len(arr)
			if iterations > maxLoopIterations {
				iterations = maxLoopIterations
			}
			for i := 0; i < iterations; i++ {
				loopData := copyMap(data)
				loopData[itemVar] = arr[i]
				loopData[itemVar+"_index"] = i

				processedBody := processConditionals(loopBody, loopData)
				processedBody = processVariables(processedBody, loopData)
				output.WriteString(processedBody)
			}
		}

		// Replace the for block with the output
		result = result[:match[0]] + output.String() + result[match[1]:]

		// If no output was generated (empty or non-array), the block is just removed
		_ = fullMatch // used for debugging
	}

	return result
}

// processConditionals handles {{if condition}}...{{else}}...{{endif}} blocks
func processConditionals(template string, data map[string]interface{}) string {
	result := template

	for {
		match := ifElsePattern.FindStringSubmatchIndex(result)
		if match == nil {
			break
		}

		// Extract condition and body
		condition := strings.TrimSpace(result[match[2]:match[3]])
		body := result[match[4]:match[5]]

		// Split body into if-part and else-part
		ifPart := body
		elsePart := ""

		elseIndex := strings.Index(body, "{{else}}")
		if elseIndex != -1 {
			ifPart = body[:elseIndex]
			elsePart = body[elseIndex+8:] // len("{{else}}") == 8
		}

		// Evaluate condition
		var output string
		if evaluateCondition(condition, data) {
			output = ifPart
		} else {
			output = elsePart
		}

		// Replace the if block with the output
		result = result[:match[0]] + output + result[match[1]:]
	}

	return result
}

// processVariables replaces {{variable}} and {{object.path}} with values
func processVariables(template string, data map[string]interface{}) string {
	return variablePattern.ReplaceAllStringFunc(template, func(match string) string {
		// Remove {{ and }}
		path := match[2 : len(match)-2]

		value := getNestedValue(data, path)
		return formatValue(value)
	})
}

// getNestedValue extracts a value from nested maps/arrays using dot notation
// Supports: "name", "user.profile.name", "items[0].name", "data.items[2].value"
func getNestedValue(data map[string]interface{}, path string) interface{} {
	if data == nil || path == "" {
		return nil
	}

	parts := splitPath(path)
	var current interface{} = data

	for _, part := range parts {
		if current == nil {
			return nil
		}

		// Check for array index: field[0]
		if idx := strings.Index(part, "["); idx != -1 {
			field := part[:idx]
			indexStr := part[idx+1 : len(part)-1]
			index, err := strconv.Atoi(indexStr)
			if err != nil {
				return nil
			}

			// Get the field first
			if field != "" {
				switch v := current.(type) {
				case map[string]interface{}:
					current = v[field]
				default:
					return nil
				}
			}

			// Then index into the array
			switch arr := current.(type) {
			case []interface{}:
				if index >= 0 && index < len(arr) {
					current = arr[index]
				} else {
					return nil
				}
			case []map[string]interface{}:
				if index >= 0 && index < len(arr) {
					current = arr[index]
				} else {
					return nil
				}
			default:
				return nil
			}
		} else {
			// Regular field access
			switch v := current.(type) {
			case map[string]interface{}:
				current = v[part]
			default:
				return nil
			}
		}
	}

	return current
}

// splitPath splits a path like "user.profile.name" or "items[0].name" into parts
func splitPath(path string) []string {
	var parts []string
	var current strings.Builder

	for i := 0; i < len(path); i++ {
		ch := path[i]

		switch ch {
		case '.':
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		case '[':
			// Include [index] with the current field name
			if current.Len() > 0 {
				current.WriteByte(ch)
				// Read until ]
				for i++; i < len(path) && path[i] != ']'; i++ {
					current.WriteByte(path[i])
				}
				if i < len(path) {
					current.WriteByte(']')
				}
			}
		default:
			current.WriteByte(ch)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// evaluateCondition evaluates a condition string against data
// Supports: "variable" (truthy), "variable == 'value'", "variable != 'value'",
// "variable > 100", "variable < 100", "variable >= 100", "variable <= 100"
func evaluateCondition(condition string, data map[string]interface{}) bool {
	condition = strings.TrimSpace(condition)

	// Parse the condition
	matches := conditionPattern.FindStringSubmatch(condition)
	if matches == nil {
		return false
	}

	varPath := matches[1]
	operator := matches[2]
	compareValue := strings.TrimSpace(matches[3])

	// Get the variable value
	value := getNestedValue(data, varPath)

	// Simple truthy check if no operator
	if operator == "" {
		return isTruthy(value)
	}

	// Remove quotes from compare value if present
	if len(compareValue) >= 2 {
		if (compareValue[0] == '\'' && compareValue[len(compareValue)-1] == '\'') ||
			(compareValue[0] == '"' && compareValue[len(compareValue)-1] == '"') {
			compareValue = compareValue[1 : len(compareValue)-1]
		}
	}

	// Perform comparison
	switch operator {
	case "==":
		return compareEqual(value, compareValue)
	case "!=":
		return !compareEqual(value, compareValue)
	case ">":
		return compareNumeric(value, compareValue) > 0
	case "<":
		return compareNumeric(value, compareValue) < 0
	case ">=":
		return compareNumeric(value, compareValue) >= 0
	case "<=":
		return compareNumeric(value, compareValue) <= 0
	}

	return false
}

// isTruthy checks if a value is "truthy"
func isTruthy(value interface{}) bool {
	if value == nil {
		return false
	}

	switch v := value.(type) {
	case bool:
		return v
	case string:
		return v != "" && v != "false" && v != "0"
	case int:
		return v != 0
	case int64:
		return v != 0
	case float64:
		return v != 0
	case []interface{}:
		return len(v) > 0
	case []map[string]interface{}:
		return len(v) > 0
	case map[string]interface{}:
		return len(v) > 0
	}

	return true
}

// compareEqual compares a value with a string for equality
func compareEqual(value interface{}, compareValue string) bool {
	if value == nil {
		return compareValue == "" || compareValue == "null" || compareValue == "nil"
	}

	switch v := value.(type) {
	case string:
		return v == compareValue
	case bool:
		return fmt.Sprintf("%v", v) == compareValue
	case int:
		return fmt.Sprintf("%d", v) == compareValue
	case int64:
		return fmt.Sprintf("%d", v) == compareValue
	case float64:
		// Try integer comparison first
		if float64(int64(v)) == v {
			return fmt.Sprintf("%d", int64(v)) == compareValue
		}
		return fmt.Sprintf("%v", v) == compareValue
	}

	return fmt.Sprintf("%v", value) == compareValue
}

// compareNumeric compares a value with a string numerically
// Returns: -1 if value < compare, 0 if equal, 1 if value > compare
func compareNumeric(value interface{}, compareValue string) int {
	// Convert value to float64
	var numValue float64
	switch v := value.(type) {
	case int:
		numValue = float64(v)
	case int64:
		numValue = float64(v)
	case float64:
		numValue = v
	case string:
		var err error
		numValue, err = strconv.ParseFloat(v, 64)
		if err != nil {
			return 0
		}
	default:
		return 0
	}

	// Convert compare value to float64
	numCompare, err := strconv.ParseFloat(compareValue, 64)
	if err != nil {
		return 0
	}

	if numValue < numCompare {
		return -1
	} else if numValue > numCompare {
		return 1
	}
	return 0
}

// formatValue converts a value to a string for template output
func formatValue(value interface{}) string {
	if value == nil {
		return ""
	}

	switch v := value.(type) {
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		// Format without trailing zeros
		if float64(int64(v)) == v {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", v)
	}
}

// copyMap creates a shallow copy of a map
func copyMap(m map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

// extractResponseMapping extracts values from API response and maps them to session variables
func extractResponseMapping(responseData map[string]interface{}, mapping map[string]string) map[string]interface{} {
	result := make(map[string]interface{})

	for varName, jsonPath := range mapping {
		value := getNestedValue(responseData, jsonPath)
		if value != nil {
			result[varName] = value
		}
	}

	return result
}

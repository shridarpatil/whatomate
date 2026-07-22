package templateutil

import (
	"fmt"
	"regexp"
	"strings"
)

// ParameterPattern matches template parameters like {{1}}, {{name}}, {{order_id}}
var ParameterPattern = regexp.MustCompile(`\{\{([^}]+)\}\}`)

// isPositionalParam returns true if the parameter name is purely numeric (e.g. "1", "2").
func isPositionalParam(name string) bool {
	for _, c := range name {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(name) > 0
}

// ValidateNoMixedParams checks that content does not mix positional ({{1}}) and
// named ({{name}}) parameters. Returns an error if both types are present.
func ValidateNoMixedParams(content string) error {
	names := ExtParamNames(content)
	if len(names) == 0 {
		return nil
	}

	hasPositional := false
	hasNamed := false
	for _, n := range names {
		if isPositionalParam(n) {
			hasPositional = true
		} else {
			hasNamed = true
		}
		if hasPositional && hasNamed {
			return fmt.Errorf("template cannot mix positional ({{1}}, {{2}}) and named ({{name}}) parameters")
		}
	}
	return nil
}

// ExtParamNames extracts parameter names from template content.
// Supports both positional ({{1}}, {{2}}) and named ({{name}}, {{order_id}}) parameters.
// Returns parameter names in order of first occurrence, without duplicates.
func ExtParamNames(content string) []string {
	matches := ParameterPattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	var names []string
	for _, match := range matches {
		if len(match) > 1 {
			name := strings.TrimSpace(match[1])
			if name != "" && !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}
	return names
}

// ResolveParamsFromMap resolves both positional and named parameters to ordered values
// using a map[string]string parameter source.
func ResolveParamsFromMap(paramNames []string, params map[string]string) []string {
	if len(paramNames) == 0 || len(params) == 0 {
		return nil
	}

	result := make([]string, len(paramNames))
	for i, name := range paramNames {
		// Try named key first
		if val, ok := params[name]; ok {
			result[i] = val
			continue
		}
		// Fall back to positional key (1-indexed)
		key := fmt.Sprintf("%d", i+1)
		if val, ok := params[key]; ok {
			result[i] = val
			continue
		}
		// Default to empty string
		result[i] = ""
	}
	return result
}

// ResolveParams resolves both positional and named parameters to ordered values
// using a map[string]any parameter source (e.g. models.JSONB).
func ResolveParams(bodyContent string, params map[string]any) []string {
	if len(params) == 0 {
		return nil
	}

	paramNames := ExtParamNames(bodyContent)
	if len(paramNames) == 0 {
		return nil
	}

	result := make([]string, len(paramNames))
	for i, name := range paramNames {
		// Try named key first
		if val, ok := params[name]; ok {
			result[i] = fmt.Sprintf("%v", val)
			continue
		}
		// Fall back to positional key (1-indexed)
		key := fmt.Sprintf("%d", i+1)
		if val, ok := params[key]; ok {
			result[i] = fmt.Sprintf("%v", val)
			continue
		}
		// Default to empty string
		result[i] = ""
	}
	return result
}

// ReplaceWithStringParams replaces {{1}}, {{2}}, {{name}}, etc. placeholders with actual values
// from a map[string]string.
func ReplaceWithStringParams(content string, params map[string]string) string {
	if content == "" || len(params) == 0 {
		return content
	}

	result := content
	paramNames := ExtParamNames(content)
	for i, name := range paramNames {
		// Try to get value by name first (works for both named and positional)
		if val, ok := params[name]; ok {
			result = strings.ReplaceAll(result, fmt.Sprintf("{{%s}}", name), val)
			continue
		}
		// Fall back to positional key (1-indexed)
		key := fmt.Sprintf("%d", i+1)
		if val, ok := params[key]; ok {
			result = strings.ReplaceAll(result, fmt.Sprintf("{{%s}}", name), val)
		}
	}
	return result
}

// ReplaceWithJSONBParams replaces both positional ({{1}}) and named ({{name}}) placeholders
// using a map[string]any parameter source. bodyContent is used to extract parameter
// names (typically the template's body content), and content is the string to perform
// replacements on.
func ReplaceWithJSONBParams(bodyContent, content string, params map[string]any) string {
	if len(params) == 0 {
		return content
	}

	paramNames := ExtParamNames(bodyContent)
	if len(paramNames) == 0 {
		return content
	}

	for i, name := range paramNames {
		// Try named key first
		var val string
		if v, ok := params[name]; ok {
			val = fmt.Sprintf("%v", v)
		} else if v, ok := params[fmt.Sprintf("%d", i+1)]; ok {
			// Fall back to positional key
			val = fmt.Sprintf("%v", v)
		}

		// Replace both named and positional placeholders
		content = strings.ReplaceAll(content, fmt.Sprintf("{{%s}}", name), val)
		content = strings.ReplaceAll(content, fmt.Sprintf("{{%d}}", i+1), val)
	}
	return content
}

// Schema translation: converts simplified frontmatter notation to JSON Schema.

package agent

import "fmt"

var primitiveTypes = map[string]bool{
	"string":  true,
	"number":  true,
	"boolean": true,
	"integer": true,
}

// TranslateSchema converts the simplified frontmatter schema notation into
// a standard JSON Schema object. If the input already has "type" and
// "properties" at the root, it is treated as valid JSON Schema and returned
// as-is.
func TranslateSchema(simplified map[string]any) (map[string]any, error) {
	// Pass-through if already a JSON Schema.
	if _, hasType := simplified["type"]; hasType {
		if _, hasProps := simplified["properties"]; hasProps {
			return simplified, nil
		}
	}

	return translateObjectFields(simplified)
}

func translateField(value any) (map[string]any, error) {
	switch v := value.(type) {
	case string:
		if primitiveTypes[v] {
			return map[string]any{"type": v}, nil
		}
		return nil, fmt.Errorf("unknown type %q", v)
	case map[string]any:
		return translateComplexField(v)
	default:
		return nil, fmt.Errorf("unsupported field value type %T", value)
	}
}

func translateComplexField(field map[string]any) (map[string]any, error) {
	fieldType, hasType := field["type"].(string)
	if !hasType {
		// No "type" key — treat as a nested object definition.
		return translateObjectFields(field)
	}

	switch fieldType {
	case "array":
		return translateArrayField(field)
	case "object":
		result := map[string]any{"type": "object"}
		if props, ok := field["properties"]; ok {
			result["properties"] = props
		}
		if req, ok := field["required"]; ok {
			result["required"] = req
		}
		return result, nil
	default:
		if primitiveTypes[fieldType] {
			return map[string]any{"type": fieldType}, nil
		}
		return nil, fmt.Errorf("unknown type %q", fieldType)
	}
}

func translateArrayField(field map[string]any) (map[string]any, error) {
	items, hasItems := field["items"]
	if !hasItems {
		return map[string]any{"type": "array"}, nil
	}

	itemSchema, translateErr := translateField(items)
	if translateErr != nil {
		return nil, fmt.Errorf("array items: %w", translateErr)
	}

	return map[string]any{
		"type":  "array",
		"items": itemSchema,
	}, nil
}

func translateObjectFields(fields map[string]any) (map[string]any, error) {
	properties := make(map[string]any, len(fields))
	required := make([]string, 0, len(fields))

	for key, value := range fields {
		prop, translateErr := translateField(value)
		if translateErr != nil {
			return nil, fmt.Errorf("field %q: %w", key, translateErr)
		}
		properties[key] = prop
		required = append(required, key)
	}

	return map[string]any{
		"type":       "object",
		"properties": properties,
		"required":   required,
	}, nil
}

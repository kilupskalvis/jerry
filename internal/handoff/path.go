package handoff

import (
	"fmt"
	"strconv"
	"strings"
)

// PathLookup walks a decoded-JSON document by the spec's deliberately tiny
// path grammar: dot segments plus [int] indexes ("a.b[0].c"). Not JSONPath —
// no filters, wildcards, or recursive descent. Returns the value rendered
// as a string (numbers without trailing zeros, everything else via %v).
func PathLookup(doc any, path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("empty path")
	}
	cur := doc
	for seg := range strings.SplitSeq(path, ".") {
		name, indexes, err := splitIndexes(seg)
		if err != nil {
			return "", err
		}
		if name != "" {
			m, ok := cur.(map[string]any)
			if !ok {
				return "", fmt.Errorf("path %q: %q is not an object", path, name)
			}
			cur, ok = m[name]
			if !ok {
				return "", fmt.Errorf("path %q: key %q not found", path, name)
			}
		}
		for _, idx := range indexes {
			arr, ok := cur.([]any)
			if !ok {
				return "", fmt.Errorf("path %q: index into non-array", path)
			}
			if idx < 0 || idx >= len(arr) {
				return "", fmt.Errorf("path %q: index %d out of range (len %d)", path, idx, len(arr))
			}
			cur = arr[idx]
		}
	}
	return renderScalar(cur), nil
}

func splitIndexes(seg string) (name string, indexes []int, err error) {
	name = seg
	for {
		open := strings.Index(name, "[")
		if open == -1 {
			return name, indexes, nil
		}
		closing := strings.Index(name[open:], "]")
		if closing == -1 {
			return "", nil, fmt.Errorf("segment %q: unterminated [", seg)
		}
		idxStr := name[open+1 : open+closing]
		idx, convErr := strconv.Atoi(idxStr)
		if convErr != nil {
			return "", nil, fmt.Errorf("segment %q: index %q is not an integer", seg, idxStr)
		}
		indexes = append(indexes, idx)
		name = name[:open] + name[open+closing+1:]
	}
}

func renderScalar(v any) string {
	switch x := v.(type) {
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case string:
		return x
	default:
		return fmt.Sprintf("%v", x)
	}
}

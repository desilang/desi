package parsebridge

import (
	"bytes"
	"fmt"
)

// extractFirstJSONObject scans data and returns the first balanced JSON object.
// It handles strings and escapes so braces inside strings don't confuse depth.
func extractFirstJSONObject(data []byte) ([]byte, error) {
	b := bytes.TrimSpace(data)
	if len(b) == 0 {
		return nil, fmt.Errorf("empty output")
	}
	// find first '{'
	start := -1
	for i := 0; i < len(b); i++ {
		if b[i] == '{' {
			start = i
			break
		}
	}
	if start < 0 {
		return nil, fmt.Errorf("no JSON object start found")
	}

	depth := 0
	inStr := false
	esc := false
	for i := start; i < len(b); i++ {
		c := b[i]
		if inStr {
			if esc {
				esc = false
				continue
			}
			if c == '\\' {
				esc = true
				continue
			}
			if c == '"' {
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			if depth == 0 && start < 0 {
				start = i
			}
			depth++
		case '}':
			if depth > 0 {
				depth--
				if depth == 0 {
					end := i + 1
					return b[start:end], nil
				}
			}
		}
	}
	return nil, fmt.Errorf("unterminated JSON object")
}

func sanitizeJSONOutput(data []byte) ([]byte, error) {
	obj, err := extractFirstJSONObject(data)
	if err != nil {
		prev := data
		if len(prev) > 200 {
			prev = prev[:200]
		}
		return nil, fmt.Errorf("parsebridge produced non-JSON or mixed output; %w\npreview: %q", err, string(prev))
	}
	return obj, nil
}

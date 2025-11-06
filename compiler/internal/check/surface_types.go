package check

import (
	"strings"

	"github.com/desilang/desi/compiler/internal/types"
)

// surfaceToType parses a small surface subset used in tests:
//
//	int | float | bool | str | none | usize | isize
//	future[T]   cptr[T]   tuple[T1, T2, ...]
//
// Returns nil if the shape is unknown.
func surfaceToType(name string) types.T {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	// Fast path for basics
	if t, ok := types.FromName(name); ok {
		return t
	}

	// Parameterized forms: <head>[<body>]
	i := strings.IndexByte(name, '[')
	if i < 0 || !strings.HasSuffix(name, "]") {
		return nil
	}
	head := strings.TrimSpace(name[:i])
	body := strings.TrimSpace(name[i+1 : len(name)-1])

	switch head {
	case "future":
		elem := surfaceToType(body)
		if elem == nil {
			return nil
		}
		return types.FutureOf(elem)
	case "cptr":
		elem := surfaceToType(body)
		if elem == nil {
			return nil
		}
		return types.CPtrOf(elem)
	case "tuple":
		if body == "" {
			return types.TupleOf()
		}
		var parts []string
		depth := 0
		start := 0
		for idx, r := range body {
			switch r {
			case '[':
				depth++
			case ']':
				if depth > 0 {
					depth--
				}
			case ',':
				if depth == 0 {
					parts = append(parts, strings.TrimSpace(body[start:idx]))
					start = idx + 1
				}
			}
		}
		parts = append(parts, strings.TrimSpace(body[start:]))

		elts := make([]types.T, 0, len(parts))
		for _, p := range parts {
			t := surfaceToType(p)
			if t == nil {
				return nil
			}
			elts = append(elts, t)
		}
		return types.TupleOf(elts...)
	default:
		return nil
	}
}

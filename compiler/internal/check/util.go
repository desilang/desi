package check

import "strings"

// tiny generic stack helpers (requires Go 1.18+)
func push[T any](s []T, v T) []T { return append(s, v) }
func pop[T any](s []T) []T       { return s[:len(s)-1] }
func top[T any](s []T) *T {
	if len(s) == 0 {
		return nil
	}
	return &s[len(s)-1]
}

/* ---------- helpers ---------- */

func mapTextType(t string) Kind {
	switch strings.TrimSpace(strings.ToLower(t)) {
	case "", "void":
		return KindVoid
	case "i32", "int", "u32":
		return KindInt
	case "bool":
		return KindBool
	case "str", "string":
		return KindStr
	default:
		return KindUnknown
	}
}

// mapTypeOrStruct returns (Kind, structName). If the textual type matches a known
// struct, returns (KindStruct, structName). Otherwise uses the builtin map.
func mapTypeOrStruct(t string, info *Info) (Kind, string) {
	trim := strings.TrimSpace(t)
	if trim == "" {
		return KindVoid, ""
	}
	// Builtins first
	if k := mapTextType(trim); k != KindUnknown {
		return k, ""
	}
	// Struct name?
	if _, ok := info.Structs[trim]; ok {
		return KindStruct, trim
	}
	// Unknown type (generic/other) — treat as unknown for now.
	return KindUnknown, ""
}

func unifyKinds(a, b Kind) (Kind, bool) {
	if a == KindUnknown {
		return b, true
	}
	if b == KindUnknown {
		return a, true
	}
	if a == b {
		return a, true
	}
	if (a == KindInt && b == KindBool) || (a == KindBool && b == KindInt) {
		return KindInt, true
	}
	// Structs never unify in this helper (we compare names elsewhere if needed)
	return KindUnknown, false
}

func _min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

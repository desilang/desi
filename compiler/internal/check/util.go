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
	return KindUnknown, false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

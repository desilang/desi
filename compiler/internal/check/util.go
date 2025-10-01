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
	case "future": // NEW: async placeholder type
		return KindFuture
	case "none":
		return KindVoid // treat 'none' like 'void' for payloads
	default:
		return KindUnknown
	}
}

// mapTypeOrStruct returns (Kind, userTypeName).
// Builtins -> (KindX, "")
// Type alias -> resolved recursively
// Struct name -> (KindStruct, "Name")
// Enum name   -> (KindEnum,   "Name")
func mapTypeOrStruct(t string, info *Info) (Kind, string) {
	trim := strings.TrimSpace(t)
	if trim == "" {
		return KindVoid, ""
	}

	// Builtins / special textuals first
	if k := mapTextType(trim); k != KindUnknown {
		return k, ""
	}

	// Type alias?
	if info != nil && info.Types != nil {
		if under, ok := info.Types[trim]; ok {
			return mapTypeOrStruct(under, info)
		}
	}

	// Struct?
	if info != nil && info.Structs != nil {
		if _, ok := info.Structs[trim]; ok {
			return KindStruct, trim
		}
	}

	// Enum?
	if info != nil && info.Enums != nil {
		if _, ok := info.Enums[trim]; ok {
			return KindEnum, trim
		}
	}

	// Unknown type (generic/other)
	return KindUnknown, ""
}

func unifyKinds(a, b Kind) (Kind, bool) {
	// Keep prior forgiving behavior:
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
	// Structs/enums/future don't cross-unify here.
	return KindUnknown, false
}

func _min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

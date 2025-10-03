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

// enforceAllowedTopName applies global (file-scope) hygiene to top-level names:
// - reserved identifiers (keywords, literals, scalar type names, std roots)
// - prelude builtins (print/…)
// - collisions with imported names visible in this file
func enforceAllowedTopName(info *Info, name, context string) error {
	n := strings.TrimSpace(name)
	if n == "" {
		return nil
	}
	if isReservedIdent(n) {
		return ErrReservedIdentifier(n, context)
	}
	if isPreludeBuiltin(n) {
		return ErrShadowBuiltin(n, context)
	}
	if info != nil && info.ImportedNames != nil && info.ImportedNames[n] {
		return ErrImportNameConflict(n, context)
	}
	return nil
}

// mapTextType maps a textual type annotation to a Kind.
// Policy:
//   - "" (no annotation)     → KindNone (function returns default to none)
//   - "none"                 → KindNone
//   - "void"                 → KindNone (back-compat; later we can emit a diag nudging to 'none')
//   - All integer spellings  → KindInt (until we introduce sized/signed kinds)
//   - "str"/"string"         → KindStr
//   - "bool"                 → KindBool
//   - "future"               → KindFuture
//   - "f32"/"f64"            → KindUnknown (not yet implemented)
func mapTextType(t string) Kind {
	switch strings.TrimSpace(strings.ToLower(t)) {
	case "":
		return KindNone
	case "none", "void":
		return KindNone

	// Integers: treat all spellings as `int` for now (non-breaking aliasing).
	case "int", "i32":
		return KindInt
	case "i8", "i16", "i64", "i128", "isize":
		return KindInt
	case "u8", "u16", "u32", "u64", "u128", "usize":
		return KindInt

	case "bool":
		return KindBool
	case "str", "string":
		return KindStr
	case "future":
		return KindFuture

	// Floats: reserved but not implemented yet.
	case "f32", "f64":
		return KindUnknown

	default:
		return KindUnknown
	}
}

// mapTypeOrStruct returns (Kind, userTypeName).
// Builtins -> (KindX, "")
// Struct name -> (KindStruct, "Name")
// Enum name   -> (KindEnum,   "Name")
func mapTypeOrStruct(t string, info *Info) (Kind, string) {
	trim := strings.TrimSpace(t)
	if trim == "" {
		return KindNone, ""
	}
	// Builtins first
	if k := mapTextType(trim); k != KindUnknown {
		return k, ""
	}
	// Struct?
	if _, ok := info.Structs[trim]; ok {
		return KindStruct, trim
	}
	// Enum?
	if _, ok := info.Enums[trim]; ok {
		return KindEnum, trim
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
	// Structs/enums never unify here; names checked in higher-level logic.
	return KindUnknown, false
}

func _min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

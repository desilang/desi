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

// mapTextType maps a textual type annotation to a Kind.
// Note: empty string means "no annotation" => KindUnknown.
// `none` is the only "no value" type. `void` is not a Desi type.
func mapTextType(t string) Kind {
  switch strings.TrimSpace(strings.ToLower(t)) {
  case "":
    return KindUnknown

  // Integers: treat all spellings as `int` for now (non-breaking aliasing).
  // We'll refine sizes/signs in a later milestone when numeric kinds land.
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
  case "none":
    return KindNone
  case "future":
    return KindFuture

  // Floats are declared in the spec but not yet implemented in checker/codegen.
  // Leave them unknown for now so we don't mis-type them as ints.
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
    return KindUnknown, ""
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

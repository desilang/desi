package check

import "fmt"

/* ---------- kinds ---------- */

type Kind int

const (
  KindUnknown Kind = iota
  KindInt
  KindStr
  KindBool
  KindVoid
  KindStruct
  KindEnum
)

func (k Kind) String() string {
  switch k {
  case KindInt:
    return "int"
  case KindStr:
    return "str"
  case KindBool:
    return "bool"
  case KindVoid:
    return "void"
  case KindStruct:
    return "struct"
  case KindEnum:
    return "enum"
  default:
    return "unknown"
  }
}

/* ---------- public info ---------- */

type FuncSig struct {
  Name   string
  Params []Kind
  Ret    Kind
}

type StructInfo struct {
  Fields map[string]string
}

type EnumInfo struct {
  Variants map[string]string
}

// PublicInfo records which top-level symbols are marked public (M10 Phase A).
// Phase A only threads the bit; Phase B will use it for visibility checks.
type PublicInfo struct {
  Funcs   map[string]bool
  Structs map[string]bool
  // Future: Consts map[string]bool, Types map[string]bool, Enums map[string]bool
}

type Info struct {
  Funcs   map[string]FuncSig
  Types   map[string]string
  Structs map[string]StructInfo
  Enums   map[string]EnumInfo
  Aliases map[string]string

  Public PublicInfo // NEW (M10): publicity table, populated during collection
}

// Warning is a lightweight compiler warning.
type Warning struct {
  Code string
  Msg  string
}

func (w Warning) String() string {
  if w.Code == "" {
    return "warning: " + w.Msg
  }
  return fmt.Sprintf("%s: %s", w.Code, w.Msg)
}

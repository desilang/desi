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
	// NOTE: We will add KindEnum when we typecheck enum *values* (M8 P2).
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
	// Field name -> textual type (e.g., "int", "str", or another struct name)
	Fields map[string]string
}

type EnumInfo struct {
	// Variant name -> textual payload type ("" => no payload)
	Variants map[string]string
}

type Info struct {
	Funcs   map[string]FuncSig // function table for arity/type checks
	Types   map[string]string  // type aliases: Name -> Underlying (textual)
	Structs map[string]StructInfo
	Enums   map[string]EnumInfo // NEW (M8): enum definitions
}

// Warning is a lightweight compiler warning.
type Warning struct {
	Code string // e.g., W0001
	Msg  string
}

func (w Warning) String() string {
	if w.Code == "" {
		return "warning: " + w.Msg
	}
	return fmt.Sprintf("%s: %s", w.Code, w.Msg)
}

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

// Minimal shape for constants; we only need kind for typing/checking now.
type ConstInfo struct {
	Kind Kind
	// (Future: track literal payload for constant folding, etc.)
}

type Info struct {
	Funcs   map[string]FuncSig
	Types   map[string]string
	Structs map[string]StructInfo
	Enums   map[string]EnumInfo
	Aliases map[string]string

	// NEW (M10 Phase B): visibility + consts
	FuncsPublic   map[string]bool
	StructsPublic map[string]bool
	Consts        map[string]ConstInfo
	ConstsPublic  map[string]bool
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

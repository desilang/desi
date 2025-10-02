package check

import "fmt"

/* ---------- kinds ---------- */

type Kind int

const (
	KindUnknown Kind = iota
	KindInt
	KindStr
	KindBool
	KindNone
	KindStruct
	KindEnum
	KindFuture // async placeholder/result carrier
)

// Back-compat: keep old symbol compiling; semantically identical to KindNone.
const KindVoid = KindNone

func (k Kind) String() string {
	switch k {
	case KindInt:
		return "int"
	case KindStr:
		return "str"
	case KindBool:
		return "bool"
	case KindNone:
		return "none"
	case KindStruct:
		return "struct"
	case KindEnum:
		return "enum"
	case KindFuture:
		return "future"
	default:
		return "unknown"
	}
}

/* ---------- public info ---------- */

type FuncSig struct {
	Name    string
	Params  []Kind
	Ret     Kind
	Async   bool // function declared with `async`
	RetElem Kind // element kind when Ret==KindFuture (e.g., future<int> -> KindInt)
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

	FuncsPublic   map[string]bool
	StructsPublic map[string]bool
	Consts        map[string]ConstInfo
	ConstsPublic  map[string]bool

	FuncsLocal  map[string]bool
	TypesPublic map[string]bool
	EnumsPublic map[string]bool

	// names introduced by imports in this file (aliases and from-items)
	ImportedNames map[string]bool
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

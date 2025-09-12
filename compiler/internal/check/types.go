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

type Info struct {
	Funcs map[string]FuncSig // function table for arity/type checks
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

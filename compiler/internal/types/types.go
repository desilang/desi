package types

import (
	"fmt"
	"strings"
)

// T is the interface implemented by all types.
// String() must return a canonical, stable representation (used in tests/diags).
type T interface {
	String() string
	isType()
}

type Kind int

const (
	InvalidKind Kind = iota
	IntKind
	FloatKind
	BoolKind
	StrKind
	NoneKind
	ListKind
	SetKind
	DictKind
	TupleKind
	FutureKind
	FuncKind
	MultiKind
)

// ----- Basic types (singletons) -----

type basic struct {
	kind Kind
	name string
}

func (b *basic) isType() {}

func (b *basic) String() string { return b.name }

var (
	Int   = &basic{IntKind, "int"}
	Float = &basic{FloatKind, "float"}
	Bool  = &basic{BoolKind, "bool"}
	Str   = &basic{StrKind, "str"}
	None  = &basic{NoneKind, "none"}
)

// ----- Constructed types -----

type List struct{ Elem T }

func (*List) isType()          {}
func (t *List) String() string { return "list[" + t.Elem.String() + "]" }

type Set struct{ Elem T }

func (*Set) isType()          {}
func (t *Set) String() string { return "set[" + t.Elem.String() + "]" }

type Dict struct{ Key, Val T }

func (*Dict) isType()          {}
func (t *Dict) String() string { return "dict[" + t.Key.String() + "," + t.Val.String() + "]" }

type Tuple struct{ Elems []T }

func (*Tuple) isType() {}
func (t *Tuple) String() string {
	parts := make([]string, len(t.Elems))
	for i, e := range t.Elems {
		parts[i] = e.String()
	}
	return "tuple[" + strings.Join(parts, ", ") + "]"
}

type Future struct{ Elem T }

func (*Future) isType()          {}
func (t *Future) String() string { return "future[" + t.Elem.String() + "]" }

type Func struct {
	Params []T
	Ret    T
}

func (*Func) isType() {}
func (t *Func) String() string {
	ps := make([]string, len(t.Params))
	for i, p := range t.Params {
		ps[i] = p.String()
	}
	return "func(" + strings.Join(ps, ", ") + ") -> " + t.Ret.String()
}

type Multi struct{ Elems []T }

func (*Multi) isType() {}
func (t *Multi) String() string {
	parts := make([]string, len(t.Elems))
	for i, e := range t.Elems {
		parts[i] = e.String()
	}
	return "multi[" + strings.Join(parts, ", ") + "]"
}

// ----- Constructors -----

func ListOf(elem T) *List { return &List{Elem: elem} }
func SetOf(elem T) *Set   { return &Set{Elem: elem} }
func DictOf(k, v T) *Dict { return &Dict{Key: k, Val: v} }
func TupleOf(elems ...T) *Tuple {
	cp := make([]T, len(elems))
	copy(cp, elems)
	return &Tuple{Elems: cp}
}
func FutureOf(elem T) *Future { return &Future{Elem: elem} }
func FuncOf(params []T, ret T) *Func {
	cp := make([]T, len(params))
	copy(cp, params)
	return &Func{Params: cp, Ret: ret}
}
func MultiOf(elems ...T) *Multi {
	cp := make([]T, len(elems))
	copy(cp, elems)
	return &Multi{Elems: cp}
}

// ----- Equality & assignability (structural, phase-1) -----

func kindOf(t T) Kind {
	switch x := t.(type) {
	case *basic:
		return x.kind
	case *List:
		return ListKind
	case *Set:
		return SetKind
	case *Dict:
		return DictKind
	case *Tuple:
		return TupleKind
	case *Future:
		return FutureKind
	case *Func:
		return FuncKind
	case *Multi:
		return MultiKind
	default:
		return InvalidKind
	}
}

func Equal(a, b T) bool {
	if a == nil || b == nil {
		return a == b
	}
	ka, kb := kindOf(a), kindOf(b)
	if ka != kb {
		return false
	}
	switch x := a.(type) {
	case *basic:
		return x.kind == b.(*basic).kind
	case *List:
		return Equal(x.Elem, b.(*List).Elem)
	case *Set:
		return Equal(x.Elem, b.(*Set).Elem)
	case *Dict:
		y := b.(*Dict)
		return Equal(x.Key, y.Key) && Equal(x.Val, y.Val)
	case *Tuple:
		y := b.(*Tuple)
		if len(x.Elems) != len(y.Elems) {
			return false
		}
		for i := range x.Elems {
			if !Equal(x.Elems[i], y.Elems[i]) {
				return false
			}
		}
		return true
	case *Future:
		return Equal(x.Elem, b.(*Future).Elem)
	case *Func:
		y := b.(*Func)
		if len(x.Params) != len(y.Params) {
			return false
		}
		for i := range x.Params {
			if !Equal(x.Params[i], y.Params[i]) {
				return false
			}
		}
		return Equal(x.Ret, y.Ret)
	case *Multi:
		y := b.(*Multi)
		if len(x.Elems) != len(y.Elems) {
			return false
		}
		for i := range x.Elems {
			if !Equal(x.Elems[i], y.Elems[i]) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// Assignable reports if a value of type src can be assigned to a destination of type dst.
// Phase-1 semantics: exact type equality only (monomorphic), except that `none` is
// assignable to itself only (no optionals yet).
func Assignable(dst, src T) bool {
	if dst == nil || src == nil {
		return false
	}
	// exact structural equality
	return Equal(dst, src)
}

// Debug helper to quickly build basic types from string names in tests.
func FromName(name string) (T, bool) {
	switch name {
	case "int":
		return Int, true
	case "float":
		return Float, true
	case "bool":
		return Bool, true
	case "str":
		return Str, true
	case "none":
		return None, true
	default:
		return nil, false
	}
}

// Must panics in tests if err != nil; small convenience.
func Must[T any](v T, ok bool) T {
	if !ok {
		panic(fmt.Sprintf("unexpected false ok in types.Must for %T", v))
	}
	return v
}

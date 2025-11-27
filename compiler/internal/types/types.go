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

	// Basic kinds
	IntKind
	FloatKind
	BoolKind
	StrKind
	NoneKind
	AnyKind  // M14: top type
	TypeKind // M14: type of a type
	StructKind
	ClassKind

	// Container kinds
	ListKind
	SetKind
	DictKind
	TupleKind

	// Async/fn kinds
	FutureKind
	FuncKind

	// Variadic result (multi-return) kind
	MultiKind

	// M9A new kind
	CPtrKind

	// Extra kinds to avoid collisions with other wrappers (internal)
	ArenaKind
	RcKind
	ArcKind
	WeakKind
)

// ----- Basic types (singletons) -----

type basic struct {
	kind Kind
	name string
}

func (b *basic) isType()        {}
func (b *basic) String() string { return b.name }
func (b *basic) Kind() Kind     { return b.kind }

// NOTE: Return exported interface T (not *basic) to avoid IDE warnings about
// “exported function with unexported return type”.
func Basic(name string, k Kind) T { return &basic{kind: k, name: name} }

var (
	Int   = &basic{kind: IntKind, name: "int"}
	Float = &basic{kind: FloatKind, name: "float"}
	Bool  = &basic{kind: BoolKind, name: "bool"}
	Str   = &basic{kind: StrKind, name: "str"}
	None  = &basic{kind: NoneKind, name: "none"}
	Any   = &basic{kind: AnyKind, name: "Any"}
	Type  = &basic{kind: TypeKind, name: "type"}

	// M9A: size-specific integers (Tier-0: treated as part of the int family)
	USize = &basic{kind: IntKind, name: "usize"}
	ISize = &basic{kind: IntKind, name: "isize"}
)

// ----- Parameterized/container types -----

type List struct{ Elem T }
type Set struct{ Elem T }
type Dict struct{ Key, Val T }
type Tuple struct{ Elems []T }
type Future struct{ Elem T }
type Func struct {
	Params   []T
	Ret      T
	Variadic bool // true if last param is *args
}
type Multi struct{ Elems []T }

// M9A: C-ABI pointer type cptr[T]
type CPtr struct{ Elem T }

type Field struct {
	Name string
	Type T
}

type Struct struct {
	Name   string
	Fields []Field
}

func (*List) isType()   {}
func (*Set) isType()    {}
func (*Dict) isType()   {}
func (*Tuple) isType()  {}
func (*Future) isType() {}
func (*Func) isType()   {}
func (*Multi) isType()  {}
func (*CPtr) isType()   {}
func (*Struct) isType() {}

func (t *List) String() string { return "list[" + t.Elem.String() + "]" }
func (t *Set) String() string  { return "set[" + t.Elem.String() + "]" }
func (t *Dict) String() string { return "dict[" + t.Key.String() + ", " + t.Val.String() + "]" }
func (t *Tuple) String() string {
	parts := make([]string, len(t.Elems))
	for i, e := range t.Elems {
		parts[i] = e.String()
	}
	return "tuple[" + strings.Join(parts, ", ") + "]"
}
func (t *Future) String() string { return "future[" + t.Elem.String() + "]" }
func (t *Func) String() string {
	ps := make([]string, len(t.Params))
	for i, p := range t.Params {
		ps[i] = p.String()
	}
	return "func(" + strings.Join(ps, ", ") + ") -> " + t.Ret.String()
}
func (t *Multi) String() string {
	parts := make([]string, len(t.Elems))
	for i, e := range t.Elems {
		parts[i] = e.String()
	}
	return "multi[" + strings.Join(parts, ", ") + "]"
}
func (t *CPtr) String() string   { return "cptr[" + t.Elem.String() + "]" }
func (t *Struct) String() string { return t.Name }

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
func FuncOf(params []T, ret T, variadic bool) *Func {
	cp := make([]T, len(params))
	copy(cp, params)
	return &Func{Params: cp, Ret: ret, Variadic: variadic}
}
func MultiOf(elems ...T) *Multi {
	cp := make([]T, len(elems))
	copy(cp, elems)
	return &Multi{Elems: cp}
}
func CPtrOf(elem T) *CPtr { return &CPtr{Elem: elem} }

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
	case *CPtr:
		return CPtrKind
	case *Struct:
		return StructKind
	// Avoid collisions with other wrappers in this package.
	// Returning distinct pseudo-kinds keeps Equal safe (no bad type assertions).
	case *Arena:
		return ArenaKind
	case *Rc:
		return RcKind
	case *Arc:
		return ArcKind
	case *Weak:
		return WeakKind
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
	case *CPtr:
		return Equal(x.Elem, b.(*CPtr).Elem)
	case *Struct:
		// Nominal equality for now (name check)
		return x.Name == b.(*Struct).Name
	default:
		return false
	}
}

// Assignable reports if a value of type src can be assigned to a destination of type dst.
// Phase-1 semantics: exact type equality only (monomorphic), except that `none` is
// assignable to itself only (no optionals yet).
func Assignable(dst, src T) bool {
	if dst == src {
		return true
	}
	if dst == nil || src == nil {
		return false
	}
	// Any accepts anything
	if kindOf(dst) == AnyKind {
		return true
	}
	// exact structural equality
	return Equal(dst, src)
}

// FromName looks up builtin named types by their canonical surface spelling.
// Note: "float" is an alias of f64 (F64 == Float).
func FromName(name string) (T, bool) {
	switch name {
	// Legacy/unsized
	case "int":
		return Int, true
	case "float", "f64":
		return F64, true

	// Basic builtins
	case "bool":
		return Bool, true
	case "str", "string":
		return Str, true
	case "none":
		return None, true
	case "Any":
		return Any, true

	// Pointer-sized ints (Tier-0: still IntKind)
	case "usize":
		return USize, true
	case "isize":
		return ISize, true

	// Sized signed ints
	case "i8":
		return I8, true
	case "i16":
		return I16, true
	case "i32":
		return I32, true
	case "i64":
		return I64, true
	case "i128":
		return I128, true

	// Sized unsigned ints
	case "u8":
		return U8, true
	case "u16":
		return U16, true
	case "u32":
		return U32, true
	case "u64":
		return U64, true
	case "u128":
		return U128, true

	// Floats
	case "f32":
		return F32, true

	default:
		return nil, false
	}
}

// Must panics in tests if ok == false; small convenience.
func Must[T any](v T, ok bool) T {
	if !ok {
		panic(fmt.Sprintf("unexpected false ok in types.Must for %T", v))
	}
	return v
}

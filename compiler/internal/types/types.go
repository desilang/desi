package types

import (
	"fmt"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
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
	EnumKind

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

	// Generics
	GenericKind
	TypeParamKind
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

	// File I/O: opaque file handle type
	File = &basic{kind: TypeKind, name: "File"}
)

// ----- Parameterized/container types -----

type List struct{ Elem T }
type Set struct{ Elem T }
type Dict struct{ Key, Val T }
type Tuple struct{ Elems []T }
type Future struct{ Elem T }
type Func struct {
	Name       string      // Optional name (for debugging/diagnostics)
	TypeParams []TypeParam // Generic type parameters
	Params     []T
	Ret        T
	Variadic   bool // true if last param is *args
	IsPub      bool // true if function is public
}
type Multi struct{ Elems []T }

// Union type for sum types: int|float, MyStruct|YourEnum, etc.
type Union struct{ Variants []T }

// Generic represents a parameterized type instantiation
// Example: Option<int> is Generic{Base: Option, Args: [Int]}
type Generic struct {
	Base T   // The generic definition (e.g., Option, Result)
	Args []T // Type arguments (e.g., [int] for Option<int>)
}

// TypeParam represents a type parameter like T, U, E
// Used during type checking of generic definitions
type TypeParam struct {
	Name string // "T", "U", "E", etc.
}

// M9A: C-ABI pointer type cptr[T]
type CPtr struct{ Elem T }

type Field struct {
	Name  string
	Type  T
	IsPub bool // public visibility (cross-file access)
	IsMut bool // mutable field (can be modified after construction)
}

type Struct struct {
	Name       string
	TypeParams []TypeParam
	Fields     []Field
}

type Variant struct {
	Name   string
	Fields []Field
	Tag    int
}

type Enum struct {
	Name       string
	TypeParams []TypeParam
	Variants   []Variant
}

// Class represents a class type with methods and dunders
type Class struct {
	Name            string
	TypeParams      []TypeParam
	Fields          []Field
	Constants       map[string]*ClassConstant // class-level constants
	StaticFields    map[string]*ClassStaticField
	Methods         map[string]*Func // method name -> function type
	StaticMethods   map[string]*Func // static method name -> function type (no self)
	ClassMethods    map[string]*Func // class method name -> function type (cls instead of self)
	Properties      map[string]*Func // property name -> function type (getter, no self strip needed for call)
	Dunders         map[string]*Func // dunder name -> function type (__new__, __repr__, etc.)
	Constructors    []*Func          // All __new__ overloads
	Base            *Class           // single inheritance (nil if no base)
	IsNested        bool             // true for nested classes
	IsAbstract      bool             // true if class has any abstract methods
	AbstractMethods map[string]bool  // set of abstract method names
	Decl            *ast.ClassDecl   // Backlink to AST for monomorphization
}

// ClassConstant represents a class-level constant
type ClassConstant struct {
	Name  string
	Type  T
	Value interface{} // ast.Expr (interface{} to avoid import cycle)
	IsPub bool
}

type ClassStaticField struct {
	Name  string
	Type  T
	IsPub bool
	IsMut bool
}

func (*List) isType()      {}
func (*Set) isType()       {}
func (*Dict) isType()      {}
func (*Tuple) isType()     {}
func (*Future) isType()    {}
func (*Func) isType()      {}
func (*Multi) isType()     {}
func (*Union) isType()     {}
func (*CPtr) isType()      {}
func (*Struct) isType()    {}
func (*Enum) isType()      {}
func (*Class) isType()     {}
func (*Generic) isType()   {}
func (*TypeParam) isType() {}

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
func (t *Union) String() string {
	parts := make([]string, len(t.Variants))
	for i, v := range t.Variants {
		parts[i] = v.String()
	}
	return strings.Join(parts, "|")
}

func (t *Generic) String() string {
	if len(t.Args) == 0 {
		return t.Base.String()
	}
	args := make([]string, len(t.Args))
	for i, a := range t.Args {
		args[i] = a.String()
	}
	return t.Base.String() + "<" + strings.Join(args, ", ") + ">"
}

func (t *TypeParam) String() string {
	return t.Name
}
func (t *CPtr) String() string   { return "cptr[" + t.Elem.String() + "]" }
func (t *Struct) String() string { return t.Name }
func (t *Enum) String() string   { return t.Name }
func (t *Class) String() string  { return t.Name }

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
func UnionOf(variants ...T) *Union {
	cp := make([]T, len(variants))
	copy(cp, variants)
	return &Union{Variants: cp}
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
	case *Enum:
		return EnumKind
	case *Class:
		return ClassKind
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
	case *Generic:
		return GenericKind
	case *TypeParam:
		return TypeParamKind
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
	case *Union:
		y := b.(*Union)
		if len(x.Variants) != len(y.Variants) {
			return false
		}
		for i := range x.Variants {
			if !Equal(x.Variants[i], y.Variants[i]) {
				return false
			}
		}
		return true
	case *CPtr:
		return Equal(x.Elem, b.(*CPtr).Elem)
	case *Struct:
		// Nominal equality for now (name check)
		return x.Name == b.(*Struct).Name
	case *Enum:
		// Nominal equality for enums (name check)
		return x.Name == b.(*Enum).Name
	case *Class:
		// Nominal equality for classes (name check)
		return x.Name == b.(*Class).Name
	case *Generic:
		y := b.(*Generic)
		if !Equal(x.Base, y.Base) {
			return false
		}
		if len(x.Args) != len(y.Args) {
			return false
		}
		for i := range x.Args {
			if !Equal(x.Args[i], y.Args[i]) {
				return false
			}
		}
		return true
	case *TypeParam:
		return x.Name == b.(*TypeParam).Name
	default:
		return false
	}
}

// Assignable reports if a value of type src can be assigned to a destination of type dst.
// Phase-1 semantics: exact type equality only (monomorphic), except that `none` is
// assignable to itself only (no optionals yet).
// Union types: src is assignable to dst if dst is a union and src matches any variant.
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
	// Union type: check if src matches any variant
	if u, ok := dst.(*Union); ok {
		for _, variant := range u.Variants {
			if Assignable(variant, src) {
				return true
			}
		}
		return false
	}

	// Special case: Empty collections (list[none], dict[none, none], set[none])
	// are assignable to any typed collection of the same kind.
	if _, ok := dst.(*List); ok {
		if srcList, ok := src.(*List); ok {
			if kindOf(srcList.Elem) == NoneKind {
				return true
			}
		}
	}
	if _, ok := dst.(*Dict); ok {
		if srcDict, ok := src.(*Dict); ok {
			if kindOf(srcDict.Key) == NoneKind && kindOf(srcDict.Val) == NoneKind {
				return true
			}
		}
	}
	if _, ok := dst.(*Set); ok {
		if srcSet, ok := src.(*Set); ok {
			if kindOf(srcSet.Elem) == NoneKind || kindOf(srcSet.Elem) == AnyKind {
				return true
			}
		}
	}

	// Special case: TypeParam can be assigned to same TypeParam name
	if dstTP, ok := dst.(*TypeParam); ok {
		if srcTP, ok := src.(*TypeParam); ok {
			return dstTP.Name == srcTP.Name
		}
	}

	// Class inheritance: src is assignable to dst if src is subclass of dst
	if dstClass, ok := dst.(*Class); ok {
		if srcClass, ok := src.(*Class); ok {
			return IsSubclass(srcClass, dstClass)
		}
	}

	// exact structural equality
	return Equal(dst, src)
}

// IsSubclass checks if sub is a subclass of base (or equal).
func IsSubclass(sub, base *Class) bool {
	curr := sub
	for curr != nil {
		if Equal(curr, base) {
			return true
		}
		curr = curr.Base
	}
	return false
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
	case "File":
		return File, true

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

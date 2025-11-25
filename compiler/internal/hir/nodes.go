package hir

// Minimal HIR nodes for Tier-0 backend: structured control, explicit Drop/DecRef,
// arena helpers, async surface ops live in async_nodes.go.
// Task H adds: function Params and frame.set/get sugar ops.

// ----- simple types (used mostly for pretty-printing/types in Tier-0) -----

type Type int

const (
	TypeUnknown Type = iota
	TypeInt
	TypeBool
	TypeStr
	TypeNone
)

func (t Type) String() string {
	switch t {
	case TypeInt:
		return "int"
	case TypeBool:
		return "bool"
	case TypeStr:
		return "str"
	case TypeNone:
		return "none"
	default:
		return "?"
	}
}

// Value is a lowered, printable value (constants or locals/temps).
type Value interface {
	isValue()
	String() string
}

// ----- simple value forms -----

type Var struct{ Name string }

func (Var) isValue()         {}
func (v Var) String() string { return v.Name }

type Temp struct{ Name string }

func (Temp) isValue()         {}
func (t Temp) String() string { return t.Name }

type ConstInt struct {
	Text string
	Type string // default "i32"
}

func (ConstInt) isValue()         {}
func (c ConstInt) String() string { return c.Text }

type ConstFloat struct {
	Text string
}

func (ConstFloat) isValue()         {}
func (c ConstFloat) String() string { return c.Text }

type ConstBool struct{ Value bool }

func (ConstBool) isValue() {}
func (c ConstBool) String() string {
	if c.Value {
		return "true"
	}
	return "false"
}

type ConstStr struct{ Text string }

func (ConstStr) isValue()         {}
func (c ConstStr) String() string { return "\"" + c.Text + "\"" }

type Undef struct{}

func (Undef) isValue()       {}
func (Undef) String() string { return "undef" }

// ----- statements -----

type Stmt interface{ isStmt() }

type Let struct {
	Name string
	Init Value       // may be nil
	Type interface{} // types.T from type checker (helps backend with reference types)
}

func (*Let) isStmt() {}

type Assign struct {
	LHS string
	RHS Value
}

func (*Assign) isStmt() {}

type Call struct {
	Fn   string
	Args []Value
	Dst  Temp // optional; empty Name => no result bound
}

func (*Call) isStmt() {}

type Ret struct{ Val Value } // nil => void ret

func (*Ret) isStmt() {}

type Drop struct{ Val Value }

func (*Drop) isStmt() {}

// Refcount ops
type IncRef struct{ Val Value }

func (*IncRef) isStmt() {}

type DecRef struct{ Val Value }

func (*DecRef) isStmt() {}

// Arena ops
type ArenaAlloc struct {
	Arena Value   // e.g., %arena
	Args  []Value // payload (type/size/initializer placeholder)
	Dst   Temp
}

func (*ArenaAlloc) isStmt() {}

type DestroyArena struct{ Arena Value }

func (*DestroyArena) isStmt() {}

// Structured control (printed by name; lowered in backend)
type If struct {
	Cond Value
	Then *Block
	Else *Block // optional
}

func (*If) isStmt() {}

type While struct {
	Cond Value
	Body *Block
}

func (*While) isStmt() {}

// ----- Task H: function params + frame sugar -----

// Param is a minimal function parameter (Tier-0 only needs the name).
type Param struct {
	Name string
	Type string // LLVM type (e.g., "i32", "ptr")
}

// FrameSet: conceptual store to a logical frame slot (e.g., "frame.x").
type FrameSet struct {
	Slot string
	Val  Value
}

func (*FrameSet) isStmt() {}

// FrameGet: conceptual load from a logical frame slot into a temp.
type FrameGet struct {
	Slot string
	Dst  Temp
}

func (*FrameGet) isStmt() {}

// ----- module/function/block containers -----

type Module struct {
	Name  string
	Funcs []*Func
}

type Func struct {
	Name    string
	Params  []Param
	RetType string // optional (e.g. "ptr", "i32")
	Blocks  []*Block
}

type Block struct {
	Name  string
	Stmts []Stmt
}

func NewBlock(name string) *Block { return &Block{Name: name} }

// ----- M14: Low-level memory ops for varargs/structs -----

type Alloca struct {
	Type  string // textual LLVM type (e.g. "i32", "{ptr, i64}")
	Count int    // number of elements (default 1)
	Dst   Temp
}

func (*Alloca) isStmt() {}

type Store struct {
	Dst Value // address (ptr)
	Val Value // value to store
}

func (*Store) isStmt() {}

type Load struct {
	Type string // type to load (e.g. "ptr")
	Src  Value  // address
	Dst  Temp
}

func (*Load) isStmt() {}

type GetElementPtr struct {
	Type    string  // type of the element being indexed (e.g. "i32" or "{ptr, i64}")
	Base    Value   // base pointer
	Indices []Value // indices
	Dst     Temp
}

func (*GetElementPtr) isStmt() {}

type BitCast struct {
	Val  Value
	Type string // target type
	Dst  Temp
}

func (*BitCast) isStmt() {}

type InsertValue struct {
	Agg   Value       // aggregate value (struct)
	Elem  Value       // element to insert
	Index int         // index to insert at
	Type  interface{} // types.T or string (LLVM type)
	Dst   Temp
}

func (*InsertValue) isStmt() {}

type ExtractValue struct {
	Agg   Value       // aggregate value (struct)
	Index int         // index to extract
	Type  interface{} // types.T or string (LLVM type)
	Dst   Temp
}

func (*ExtractValue) isStmt() {}

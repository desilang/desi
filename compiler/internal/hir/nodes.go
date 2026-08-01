package hir

import "bytes"

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

type ListLit struct {
	Elems []Value
}

func (*ListLit) isValue() {}

func (x *ListLit) String() string {
	var buf bytes.Buffer
	buf.WriteString("[")
	for i, e := range x.Elems {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(e.String())
	}
	buf.WriteString("]")
	return buf.String()
}

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

// ConstNull represents a null pointer constant
type ConstNull struct{}

func (ConstNull) isValue()       {}
func (ConstNull) String() string { return "null" }

// FuncRef represents a reference to a function (for function pointers)
type FuncRef struct{ Name string }

func (FuncRef) isValue()         {}
func (f FuncRef) String() string { return "@" + f.Name }

type Undef struct{}

func (Undef) isValue()       {}
func (Undef) String() string { return "undef" }

// ----- statements -----

type Stmt interface{ isStmt() }

// Binary operation: dst = lhs op rhs
type BinaryOp struct {
	Op   string // Operator: "+", "-", "*", "/", "%", "==", "<", ">", "<=", ">=", "!="
	LHS  Value
	RHS  Value
	Dst  Temp
	Type string // LLVM type hint (e.g., "i32", "i64", "i1")
}

func (*BinaryOp) isStmt() {}

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
	Dst  Temp // optional; empty Name => no result bound
	Fn   string
	Args []Value
	Type string // Return type, default "i32"
}

func (*Call) isStmt() {}

// TailCall marks a self-recursive call in tail position.
// These are transformed into loops by the LLVM backend.
type TailCall struct {
	Fn   string  // Function name (must match current function)
	Args []Value // New values for parameters on next iteration
}

func (*TailCall) isStmt() {}

type Ret struct{ Val Value } // nil => void ret

func (*Ret) isStmt() {}

type Cast struct {
	Dst  Temp
	Src  Value
	Type string // Target type, e.g. "ptr"
}

func (*Cast) isStmt() {}

// Select: dst = cond ? then : else (LLVM select instruction)
type Select struct {
	Dst  Temp
	Cond Value  // i1 condition
	Then Value  // value if true
	Else Value  // value if false
	Type string // result type
}

func (*Select) isStmt() {}

type Drop struct {
	Val  Value
	Type interface{} // types.T from type checker (helps backend with cleanup)
}

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
	// ThenTail and ElseTail are the blocks each branch's control actually ends
	// in, which is not the branch's entry block once the branch contains a loop:
	// a loop hands its exit block to the lowerer, and everything after the loop
	// — including nothing at all — belongs to that block. The backend appends
	// the branch to merge here. Nil means the branch never left its entry block.
	ThenTail *Block
	ElseTail *Block
}

func (*If) isStmt() {}

// Jump: unconditional branch to a target block
type Jump struct {
	Target *Block
}

func (*Jump) isStmt() {}

type While struct {
	Cond      Value  // Final condition value (temp holding boolean result)
	CondBlock *Block // Block that evaluates Cond (for re-evaluation in loop header)
	Body      *Block
	// Exit is the block control reaches when the condition fails. The lowerer
	// owns it so that 'break' can name it as a Jump target; the backend falls
	// back to synthesising a label when it is nil.
	Exit *Block
	// Latch closes the iteration: scope drops, and for a desugared 'for' the
	// index increment. It is the back-edge to CondBlock, and the target of
	// 'continue' — which is why the increment cannot live at the end of Body.
	// Nil means Body branches straight back to CondBlock.
	Latch *Block
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
	Name    string
	Funcs   []*Func
	Globals map[string]Global // Global constants: name -> value/type
}

// Global represents a module-level constant
type Global struct {
	Name    string
	Type    string // LLVM type (e.g., "i32", "ptr")
	Value   string // Initial value (e.g., "42", "3.14")
	IsConst bool   // true for immutable globals
}

type Func struct {
	Name    string
	Params  []Param
	RetType string // optional (e.g. "ptr", "i32")
	Blocks  []*Block
	Origin  interface{} // *ast.FuncDecl (using interface{} to avoid import cycle if needed, but ast is likely fine)
	Inline  bool        // true if @inline decorator present (emit LLVM alwaysinline)
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
	Type     string // type to load (e.g. "ptr")
	Src      Value  // address
	Dst      Temp
	DesiType interface{} // types.T from type checker
}

func (*Load) isStmt() {}

// AddressOf takes the address of a value (for FFI pointer semantics)
type AddressOf struct {
	Src  Value  // value to take address of
	Dst  Temp   // destination temp (will be ptr)
	Type string // type of the value being addressed
}

func (*AddressOf) isStmt() {}

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

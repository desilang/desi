package hir

// Minimal HIR nodes (M7A/B/C): structured control, explicit Drop, DecRef,
// and arena-specific ops.

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

type ConstInt struct{ Text string }

func (ConstInt) isValue()         {}
func (c ConstInt) String() string { return c.Text }

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

// ----- statements -----

type Stmt interface{ isStmt() }

type Let struct {
	Name string
	Init Value // may be nil
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
	Dst  Temp
}

func (*Call) isStmt() {}

type Ret struct{ Val Value }

func (*Ret) isStmt() {}

type Drop struct{ Val Value }

func (*Drop) isStmt() {}

// Refcount ops (M7B)
type IncRef struct{ Val Value }

func (*IncRef) isStmt() {}

type DecRef struct{ Val Value }

func (*DecRef) isStmt() {}

// Arena ops (M7C)
type ArenaAlloc struct {
	Arena Value   // e.g., %arena
	Args  []Value // payload (type/size/initializer placeholder)
	Dst   Temp
}

func (*ArenaAlloc) isStmt() {}

type DestroyArena struct{ Arena Value }

func (*DestroyArena) isStmt() {}

type If struct {
	Cond Value
	Then *Block
	Else *Block
}

func (*If) isStmt() {}

type While struct {
	Cond Value
	Body *Block
}

func (*While) isStmt() {}

// ----- module/func/block -----

type Module struct {
	Name  string
	Funcs []*Func
}

type Func struct {
	Name   string
	Blocks []*Block
}

type Block struct {
	Name  string
	Stmts []Stmt
}

func NewBlock(name string) *Block { return &Block{Name: name} }

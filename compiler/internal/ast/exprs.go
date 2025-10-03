package ast

/*** EXPRESSIONS ***/

type Expr interface {
	Node
	expr()
}

type IdentExpr struct {
	Name string
	Span Span
}

func (*IdentExpr) node() {}
func (*IdentExpr) expr() {}

type IntLit struct {
	Value string
	Span  Span
}

func (*IntLit) node() {}
func (*IntLit) expr() {}

/*** NEW: Float literal ***/
type FloatLit struct {
	Value string
	Span  Span
}

func (*FloatLit) node() {}
func (*FloatLit) expr() {}

type StrLit struct {
	Value string
	Span  Span
}

func (*StrLit) node() {}
func (*StrLit) expr() {}

type BoolLit struct {
	Value bool
	Span  Span
}

func (*BoolLit) node() {}
func (*BoolLit) expr() {}

type CallExpr struct {
	Callee Expr
	Args   []Expr
	Span   Span
}

func (*CallExpr) node() {}
func (*CallExpr) expr() {}

type IndexExpr struct {
	Seq   Expr
	Index Expr
	Span  Span
}

func (*IndexExpr) node() {}
func (*IndexExpr) expr() {}

type FieldExpr struct {
	X    Expr
	Name string
	Span Span
}

func (*FieldExpr) node() {}
func (*FieldExpr) expr() {}

type UnaryExpr struct {
	Op   string
	X    Expr
	Span Span
}

func (*UnaryExpr) node() {}
func (*UnaryExpr) expr() {}

type BinaryExpr struct {
	Op    string
	Left  Expr
	Right Expr
	Span  Span
}

func (*BinaryExpr) node() {}
func (*BinaryExpr) expr() {}

/*** NEW (M11): await expression ***/

type AwaitExpr struct {
	Expr Expr
	Span Span
}

func (*AwaitExpr) node() {}
func (*AwaitExpr) expr() {}

/*** NEW: Struct literal ***/

type StructLitField struct {
	Name  string
	Value Expr
	Span  Span
}

type StructLit struct {
	Name   string           // struct type name, e.g., "User"
	Fields []StructLitField // designated fields
	Span   Span
}

func (*StructLit) node() {}
func (*StructLit) expr() {}

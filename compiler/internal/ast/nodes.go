package ast

import "github.com/desilang/desi/compiler/internal/diag"

// Every node carries a source Span.
type Node interface{ SpanOf() diag.Span }

/* ---------- Module / Decls ---------- */

type Module struct {
	File  string
	Decls []Decl
	Span  diag.Span
}

func (m *Module) SpanOf() diag.Span { return m.Span }

type Decl interface {
	Node
	isDecl()
}

type FuncDecl struct {
	Async   bool
	Pub     bool
	Name    Ident
	Params  []Param
	RetType *TypeName // optional
	Body    *Block    // nil if just a signature + NL
	// Decorators holds any @decorators that immediately preceded this decl.
	Decorators []*Decorator
	// Doc holds the first triple-quoted string ("""...""") from the function body,
	// if present. The parser removes that DocStringStmt from the Block.
	Doc  *StrLit
	Span diag.Span
}

func (*FuncDecl) isDecl()             {}
func (d *FuncDecl) SpanOf() diag.Span { return d.Span }

type Param struct {
	Name     Ident
	Type     *TypeName // optional
	Default  Expr      // optional
	Mode     ParamMode // default is ParamMove (zero value)
	Variadic bool      // true if *args
	Span     diag.Span
}

/* ---------- Statements ---------- */

type Stmt interface {
	Node
	isStmt()
}

type Block struct {
	Stmts []Stmt
	Span  diag.Span
}

func (b *Block) SpanOf() diag.Span { return b.Span }

type LetStmt struct {
	Mutable bool
	Name    Ident
	Type    *TypeName // optional
	Value   Expr
	Span    diag.Span
}

func (*LetStmt) isStmt()             {}
func (s *LetStmt) SpanOf() diag.Span { return s.Span }

type ReturnStmt struct {
	Value Expr // optional
	Span  diag.Span
}

func (*ReturnStmt) isStmt()             {}
func (s *ReturnStmt) SpanOf() diag.Span { return s.Span }

type ExprStmt struct {
	Expr Expr
	Span diag.Span
}

func (*ExprStmt) isStmt()             {}
func (s *ExprStmt) SpanOf() diag.Span { return s.Span }

/* ---------- Expressions ---------- */

type Expr interface {
	Node
	isExpr()
}

type Ident struct {
	Name string
	Span diag.Span
}

func (*Ident) isExpr()             {}
func (x *Ident) SpanOf() diag.Span { return x.Span }

type IntLit struct {
	Text string
	Span diag.Span
}

func (*IntLit) isExpr()             {}
func (x *IntLit) SpanOf() diag.Span { return x.Span }

type FloatLit struct {
	Text string
	Span diag.Span
}

func (*FloatLit) isExpr()             {}
func (x *FloatLit) SpanOf() diag.Span { return x.Span }

// StrLit tracks whether it was a triple-quoted (long) string.
// Long == true when the token was LONGSTR (scanner recognized """...""").
// Value is populated for F-string parts (FSTR_PART tokens) to store the literal text.
type StrLit struct {
	Long  bool   // true for """...""", false for "..." or f"..."
	Value string // populated for F-string parts, empty otherwise
	Span  diag.Span
}

func (*StrLit) isExpr()             {}
func (x *StrLit) SpanOf() diag.Span { return x.Span }

type FString struct {
	Parts []Expr // StrLit or other Exprs
	Span  diag.Span
}

func (*FString) isExpr()             {}
func (x *FString) SpanOf() diag.Span { return x.Span }

type BoolLit struct {
	Value bool
	Span  diag.Span
}

func (*BoolLit) isExpr()             {}
func (x *BoolLit) SpanOf() diag.Span { return x.Span }

type NoneLit struct {
	Span diag.Span
}

func (*NoneLit) isExpr()             {}
func (x *NoneLit) SpanOf() diag.Span { return x.Span }

type DictLit struct {
	Keys   []Expr
	Values []Expr
	Span   diag.Span
}

func (*DictLit) isExpr()             {}
func (x *DictLit) SpanOf() diag.Span { return x.Span }

type SetLit struct {
	Elems []Expr
	Span  diag.Span
}

func (*SetLit) isExpr()             {}
func (x *SetLit) SpanOf() diag.Span { return x.Span }

type TupleLit struct {
	Elems []Expr
	Span  diag.Span
}

func (*TupleLit) isExpr()             {}
func (x *TupleLit) SpanOf() diag.Span { return x.Span }

type UnaryExpr struct {
	Op   string // "-", "!", "not", "await"
	X    Expr
	Span diag.Span
}

func (*UnaryExpr) isExpr()             {}
func (x *UnaryExpr) SpanOf() diag.Span { return x.Span }

type BinaryExpr struct {
	Op   string // "**", "*", "/", "%", "+", "-", "^", "|", "<", "<=", ">", ">=", "==", "!=", "|>", "and", "or"
	Lhs  Expr
	Rhs  Expr
	Span diag.Span
}

func (*BinaryExpr) isExpr()             {}
func (x *BinaryExpr) SpanOf() diag.Span { return x.Span }

// CallArg represents a single argument in a call.
// Name == nil for positional args. Star marks a future '*expr' expansion (wired in F).
type CallArg struct {
	Name *Ident // nil for positional
	Expr Expr
	Star bool
}

type CallExpr struct {
	Callee   Expr
	Args     []Expr    // legacy positional-only list (kept for back-compat)
	ArgNodes []CallArg // canonical argument list with names
	Span     diag.Span
}

func (*CallExpr) isExpr()             {}
func (x *CallExpr) SpanOf() diag.Span { return x.Span }

type IndexExpr struct {
	X    Expr
	Idx  Expr
	Span diag.Span
}

func (*IndexExpr) isExpr()             {}
func (x *IndexExpr) SpanOf() diag.Span { return x.Span }

type FieldExpr struct {
	X    Expr
	Name Ident
	Span diag.Span
}

func (*FieldExpr) isExpr()             {}
func (x *FieldExpr) SpanOf() diag.Span { return x.Span }

/* ---------- Types (minimal) ---------- */

type TypeName struct {
	Name string // "Foo" or "a.b.C"
	Span diag.Span
}

/* ---------- Helpers ---------- */

// JoinSpan returns a Span spanning [a,b] (or a if b is zero/empty).
func JoinSpan(a, b diag.Span) diag.Span {
	out := a
	if b.Start.Line != 0 {
		out.End = b.End
	}
	return out
}

package ast

/*** STATEMENTS ***/

type Stmt interface {
	Node
	stmt()
}

/*
Parallel lets:

	let a, b:int, c = 1, 2, 3
	let mut (x: A, y: B): Pair[A,B] = f(), g()
*/
type LetBind struct {
	Name string
	Type string
	Span Span // span of this single binder (optional)
}

type LetStmt struct {
	Mutable   bool
	Binds     []LetBind // one or more names
	GroupType string    // optional overall type after ')' when LHS was parenthesized
	Values    []Expr    // one or more expressions
	Span      Span      // span of the whole stmt (from 'let' to newline)
}

func (LetStmt) node() {}
func (LetStmt) stmt() {}

/*
Parallel assignment:

	a, b := b, a
	single-name still represented the same: Names=[x], Exprs=[...]
*/
type AssignStmt struct {
	Names []string
	Exprs []Expr
	Span  Span // whole assignment
}

func (AssignStmt) node() {}
func (AssignStmt) stmt() {}

type ReturnStmt struct {
	Expr Expr // may be nil
	Span Span
}

func (ReturnStmt) node() {}
func (ReturnStmt) stmt() {}

type ExprStmt struct {
	Expr Expr
	Span Span
}

func (ExprStmt) node() {}
func (ExprStmt) stmt() {}

type IfStmt struct {
	Cond  Expr
	Then  []Stmt
	Elifs []ElseIf
	Else  []Stmt // optional; nil if absent
	Span  Span
}

func (IfStmt) node() {}
func (IfStmt) stmt() {}

type ElseIf struct {
	Cond Expr
	Body []Stmt
	Span Span
}

type WhileStmt struct {
	Cond Expr
	Body []Stmt
	Span Span
}

func (WhileStmt) node() {}
func (WhileStmt) stmt() {}

type DeferStmt struct {
	Call Expr // must be a call expression in Stage-0
	Span Span
}

func (DeferStmt) node() {}
func (DeferStmt) stmt() {}

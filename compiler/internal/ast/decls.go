package ast

/*** DECLS ***/

type FuncDecl struct {
	Name   string
	Params []Param
	Ret    string // textual type for now
	Body   []Stmt

	Span Span // span of the whole decl (from 'def' to end)
}

func (FuncDecl) node() {}
func (FuncDecl) decl() {}

type Param struct {
	Name string
	Type string
	Span Span // optional: 'name: type'
}

/*** NEW: TypeDecl (M6) ***/

type TypeDecl struct {
	Name       string
	Underlying string // textual underlying type
	Span       Span   // whole decl span (optional for now)
}

func (TypeDecl) node() {}
func (TypeDecl) decl() {}

/*** NEW: StructDecl (M7 P1) ***/

type StructDecl struct {
	Name   string
	Fields []Field
	Span   Span // whole struct span
}

func (StructDecl) node() {}
func (StructDecl) decl() {}

type Field struct {
	Name string
	Type string
	Span Span // 'name: type' span
}

/*** NEW: EnumDecl (M8 P1) ***/

type EnumDecl struct {
	Name     string
	Variants []EnumVariant
	Span     Span // whole enum span
}

func (EnumDecl) node() {}
func (EnumDecl) decl() {}

type EnumVariant struct {
	Name    string // variant name (e.g., Ok, Err)
	Payload string // textual payload type; "" means no payload (aka `none`)
	Span    Span   // span of this variant line
}

/*** NEW: Match (M8 P3) ***/

type Pattern struct {
	Variant string // Variant name (e.g., Ok, None)
	Bind    string // payload binding name ("" when none)
	Span    Span
}

type MatchArm struct {
	Pat  Pattern
	Body []Stmt
	Span Span
}

type MatchStmt struct {
	Scrut Expr       // scrutinee (Stage-1: we expect IdentExpr, but keep Expr)
	Arms  []MatchArm // one or more arms
	Span  Span
}

func (MatchStmt) node() {}
func (MatchStmt) stmt() {}

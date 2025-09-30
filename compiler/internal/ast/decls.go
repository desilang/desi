package ast

/*** DECLS ***/

type FuncDecl struct {
  Name   string
  Params []Param
  Ret    string // textual type for now
  Body   []Stmt
  Pub    bool // NEW (M10): exported
  Async  bool // NEW (M11): async function produces Future[Ret]

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
  Pub        bool   // NEW (M10): exported
  Span       Span   // whole decl span (optional for now)
}

func (TypeDecl) node() {}
func (TypeDecl) decl() {}

/*** NEW: StructDecl (M7 P1) ***/

type StructDecl struct {
  Name   string
  Fields []Field
  Pub    bool // NEW (M10): exported
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
  Pub      bool // NEW (M10): exported
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

/*** NEW (M10): Top-level constant declaration ***/

// ConstDecl represents a top-level constant:
//
//	[pub] let NAME (":" Type)? "=" <expr>
//
// Notes:
//   - Parser will set Pub=true when prefixed with `pub`.
//   - Mutable must be false for valid public constants; if parser sees `pub let mut`
//     it will still record it as Mutable=true so the checker can emit DTE0012.
type ConstDecl struct {
  Name    string
  Type    string // optional type annotation
  Value   Expr   // single expression
  Pub     bool   // exported?
  Mutable bool   // should be false for public constants
  Span    Span
}

func (ConstDecl) node() {}
func (ConstDecl) decl() {}

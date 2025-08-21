package ast

import (
	"strings"

	"github.com/desilang/desi/compiler/internal/term"
)

/*** NODES ***/

type Node interface{ node() }

type File struct {
	Pkg     *PackageDecl
	Imports []ImportDecl
	Decls   []Decl
}

func (File) node() {}

type PackageDecl struct{ Name string }

func (PackageDecl) node() {}

type ImportDecl struct {
	Path    string // e.g. "std.io"
	Aliases []string
}

func (ImportDecl) node() {}

type Decl interface {
	Node
	decl()
}

type FuncDecl struct {
	Name   string
	Params []Param
	Ret    string // textual type for now
	Body   []Stmt
}

func (FuncDecl) node() {}
func (FuncDecl) decl() {}

type Param struct {
	Name string
	Type string
}

/*** EXPRESSIONS ***/

type Expr interface {
	Node
	expr()
}

type IdentExpr struct{ Name string }

func (*IdentExpr) node() {}
func (*IdentExpr) expr() {}

type IntLit struct{ Value string }

func (*IntLit) node() {}
func (*IntLit) expr() {}

type StrLit struct{ Value string }

func (*StrLit) node() {}
func (*StrLit) expr() {}

type BoolLit struct{ Value bool }

func (*BoolLit) node() {}
func (*BoolLit) expr() {}

type CallExpr struct {
	Callee Expr
	Args   []Expr
}

func (*CallExpr) node() {}
func (*CallExpr) expr() {}

type IndexExpr struct {
	Seq   Expr
	Index Expr
}

func (*IndexExpr) node() {}
func (*IndexExpr) expr() {}

type FieldExpr struct {
	X    Expr
	Name string
}

func (*FieldExpr) node() {}
func (*FieldExpr) expr() {}

type UnaryExpr struct {
	Op string
	X  Expr
}

func (*UnaryExpr) node() {}
func (*UnaryExpr) expr() {}

type BinaryExpr struct {
	Op    string
	Left  Expr
	Right Expr
}

func (*BinaryExpr) node() {}
func (*BinaryExpr) expr() {}

/*** STATEMENTS ***/

type Stmt interface {
	Node
	stmt()
}

type LetStmt struct {
	Mutable bool
	Name    string
	Expr    Expr
}

func (LetStmt) node() {}
func (LetStmt) stmt() {}

type AssignStmt struct {
	Name string
	Expr Expr
}

func (AssignStmt) node() {}
func (AssignStmt) stmt() {}

type ReturnStmt struct {
	Expr Expr // may be nil
}

func (ReturnStmt) node() {}
func (ReturnStmt) stmt() {}

type ExprStmt struct {
	Expr Expr
}

func (ExprStmt) node() {}
func (ExprStmt) stmt() {}

/*** DUMP (pretty outline for CLI) ***/

func DumpFile(f *File) string {
	var b strings.Builder
	if f.Pkg != nil {
		term.Bprintf(&b, "package %s\n", f.Pkg.Name)
	}
	for _, im := range f.Imports {
		term.Bprintf(&b, "import %s\n", im.Path)
	}
	for _, d := range f.Decls {
		switch fn := d.(type) {
		case *FuncDecl:
			term.Bprintf(&b, "\ndef %s(", fn.Name)
			for i, p := range fn.Params {
				if i > 0 {
					b.WriteString(", ")
				}
				term.Bprintf(&b, "%s: %s", p.Name, p.Type)
			}
			term.Bprintf(&b, ") -> %s:\n", orDefault(fn.Ret, "void"))
			for _, s := range fn.Body {
				switch st := s.(type) {
				case *LetStmt:
					if st.Mutable {
						term.Bprintf(&b, "  let mut %s = %s\n", st.Name, exprString(st.Expr))
					} else {
						term.Bprintf(&b, "  let %s = %s\n", st.Name, exprString(st.Expr))
					}
				case *AssignStmt:
					term.Bprintf(&b, "  %s := %s\n", st.Name, exprString(st.Expr))
				case *ReturnStmt:
					if st.Expr == nil {
						term.Bprintf(&b, "  return\n")
					} else {
						term.Bprintf(&b, "  return %s\n", exprString(st.Expr))
					}
				case *ExprStmt:
					term.Bprintf(&b, "  %s\n", exprString(st.Expr))
				}
			}
		}
	}
	return b.String()
}

func orDefault(s, d string) string {
	if strings.TrimSpace(s) == "" {
		return d
	}
	return s
}

func exprString(e Expr) string {
	switch v := e.(type) {
	case *IdentExpr:
		return v.Name
	case *IntLit:
		return v.Value
	case *StrLit:
		return v.Value
	case *BoolLit:
		if v.Value {
			return "true"
		}
		return "false"
	case *CallExpr:
		var parts []string
		for _, a := range v.Args {
			parts = append(parts, exprString(a))
		}
		return exprString(v.Callee) + "(" + strings.Join(parts, ", ") + ")"
	case *IndexExpr:
		return exprString(v.Seq) + "[" + exprString(v.Index) + "]"
	case *FieldExpr:
		return exprString(v.X) + "." + v.Name
	case *UnaryExpr:
		return v.Op + " " + exprString(v.X)
	case *BinaryExpr:
		// Parenthesize to make precedence obvious in the dump
		return "(" + exprString(v.Left) + " " + v.Op + " " + exprString(v.Right) + ")"
	default:
		return "<expr>"
	}
}

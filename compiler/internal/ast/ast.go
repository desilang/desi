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

/*** STATEMENTS ***/

type Stmt interface {
	Node
	stmt()
}

type LetStmt struct {
	Mutable bool
	Name    string
	Expr    string // textual expr for now
}

func (LetStmt) node() {}
func (LetStmt) stmt() {}

type AssignStmt struct {
	Name string
	Expr string
}

func (AssignStmt) node() {}
func (AssignStmt) stmt() {}

type ReturnStmt struct {
	Expr string // may be empty
}

func (ReturnStmt) node() {}
func (ReturnStmt) stmt() {}

type ExprStmt struct {
	Expr string
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
						term.Bprintf(&b, "  let mut %s = %s\n", st.Name, st.Expr)
					} else {
						term.Bprintf(&b, "  let %s = %s\n", st.Name, st.Expr)
					}
				case *AssignStmt:
					term.Bprintf(&b, "  %s := %s\n", st.Name, st.Expr)
				case *ReturnStmt:
					if st.Expr == "" {
						term.Bprintf(&b, "  return\n")
					} else {
						term.Bprintf(&b, "  return %s\n", st.Expr)
					}
				case *ExprStmt:
					term.Bprintf(&b, "  %s\n", st.Expr)
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

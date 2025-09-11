package ast

import (
	"fmt"
	"strings"
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

/*
Parallel lets:

	let a, b:int, c = 1, 2, 3
	let mut (x: A, y: B): Pair[A,B] = f(), g()
*/
type LetBind struct {
	Name string
	Type string // optional per-name type annotation (textual)
}

type LetStmt struct {
	Mutable   bool
	Binds     []LetBind // one or more names
	GroupType string    // optional overall type after ')' when LHS was parenthesized
	Values    []Expr    // one or more expressions
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

type IfStmt struct {
	Cond  Expr
	Then  []Stmt
	Elifs []ElseIf
	Else  []Stmt // optional; nil if absent
}

func (IfStmt) node() {}
func (IfStmt) stmt() {}

type ElseIf struct {
	Cond Expr
	Body []Stmt
}

type WhileStmt struct {
	Cond Expr
	Body []Stmt
}

func (WhileStmt) node() {}
func (WhileStmt) stmt() {}

type DeferStmt struct {
	Call Expr // must be a call expression in Stage-0
}

func (DeferStmt) node() {}
func (DeferStmt) stmt() {}

/*** DUMP (pretty outline for CLI) ***/

func DumpFile(f *File) string {
	var b strings.Builder
	if f.Pkg != nil {
		fmt.Fprintf(&b, "package %s\n", f.Pkg.Name)
	}
	for _, im := range f.Imports {
		fmt.Fprintf(&b, "import %s\n", im.Path)
	}
	for _, d := range f.Decls {
		switch fn := d.(type) {
		case *FuncDecl:
			fmt.Fprintf(&b, "\ndef %s(", fn.Name)
			for i, p := range fn.Params {
				if i > 0 {
					b.WriteString(", ")
				}
				fmt.Fprintf(&b, "%s: %s", p.Name, p.Type)
			}
			fmt.Fprintf(&b, ") -> %s:\n", orDefault(fn.Ret, "void"))
			for _, s := range fn.Body {
				switch st := s.(type) {
				case *LetStmt:
					// let / let mut
					if st.Mutable {
						fmt.Fprintf(&b, "  let mut ")
					} else {
						fmt.Fprintf(&b, "  let ")
					}
					// print binds (show per-name types when present)
					for i, bd := range st.Binds {
						if i > 0 {
							b.WriteString(", ")
						}
						if strings.TrimSpace(bd.Type) == "" {
							fmt.Fprintf(&b, "%s", bd.Name)
						} else {
							fmt.Fprintf(&b, "%s: %s", bd.Name, bd.Type)
						}
					}
					if strings.TrimSpace(st.GroupType) != "" {
						fmt.Fprintf(&b, " : %s", st.GroupType)
					}
					b.WriteString(" = ")
					for i, e := range st.Values {
						if i > 0 {
							b.WriteString(", ")
						}
						b.WriteString(exprString(e))
					}
					b.WriteString("\n")
				case *AssignStmt:
					fmt.Fprintf(&b, "  ")
					for i, n := range st.Names {
						if i > 0 {
							b.WriteString(", ")
						}
						b.WriteString(n)
					}
					b.WriteString(" := ")
					for i, e := range st.Exprs {
						if i > 0 {
							b.WriteString(", ")
						}
						b.WriteString(exprString(e))
					}
					b.WriteString("\n")
				case *ReturnStmt:
					if st.Expr == nil {
						fmt.Fprintf(&b, "  return\n")
					} else {
						fmt.Fprintf(&b, "  return %s\n", exprString(st.Expr))
					}
				case *ExprStmt:
					fmt.Fprintf(&b, "  %s\n", exprString(st.Expr))
				case *IfStmt:
					fmt.Fprintf(&b, "  if %s:\n", exprString(st.Cond))
					for _, s2 := range st.Then {
						fmt.Fprintf(&b, "    %s\n", stmtString(s2))
					}
					for _, e := range st.Elifs {
						fmt.Fprintf(&b, "  elif %s:\n", exprString(e.Cond))
						for _, s2 := range e.Body {
							fmt.Fprintf(&b, "    %s\n", stmtString(s2))
						}
					}
					if st.Else != nil {
						fmt.Fprintf(&b, "  else:\n")
						for _, s2 := range st.Else {
							fmt.Fprintf(&b, "    %s\n", stmtString(s2))
						}
					}
				case *WhileStmt:
					fmt.Fprintf(&b, "  while %s:\n", exprString(st.Cond))
					for _, s2 := range st.Body {
						fmt.Fprintf(&b, "    %s\n", stmtString(s2))
					}
				case *DeferStmt:
					fmt.Fprintf(&b, "  defer %s\n", exprString(st.Call))
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
		return "(" + exprString(v.Left) + " " + v.Op + " " + exprString(v.Right) + ")"
	default:
		return "<expr>"
	}
}

func stmtString(s Stmt) string {
	switch st := s.(type) {
	case *LetStmt:
		var b strings.Builder
		if st.Mutable {
			b.WriteString("let mut ")
		} else {
			b.WriteString("let ")
		}
		for i, bd := range st.Binds {
			if i > 0 {
				b.WriteString(", ")
			}
			if strings.TrimSpace(bd.Type) == "" {
				b.WriteString(bd.Name)
			} else {
				fmt.Fprintf(&b, "%s: %s", bd.Name, bd.Type)
			}
		}
		if strings.TrimSpace(st.GroupType) != "" {
			fmt.Fprintf(&b, " : %s", st.GroupType)
		}
		b.WriteString(" = ")
		for i, e := range st.Values {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(exprString(e))
		}
		return b.String()
	case *AssignStmt:
		var b strings.Builder
		for i, n := range st.Names {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(n)
		}
		b.WriteString(" := ")
		for i, e := range st.Exprs {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(exprString(e))
		}
		return b.String()
	case *ReturnStmt:
		if st.Expr == nil {
			return "return"
		}
		return "return " + exprString(st.Expr)
	case *ExprStmt:
		return exprString(st.Expr)
	case *IfStmt:
		return "if …:"
	case *WhileStmt:
		return "while …:"
	case *DeferStmt:
		return "defer " + exprString(st.Call)
	default:
		return "<stmt>"
	}
}

package ast

import (
	"fmt"
	"io"
)

// Print renders a stable, tab-indented AST (no colors) for dumps/tests.
func Print(w io.Writer, n Node) {
	pp{w: w}.node(n, 0)
}

type pp struct{ w io.Writer }

func (p pp) wr(s string, args ...any) { _, _ = fmt.Fprintf(p.w, s, args...) }

func (p pp) tabs(n int) {
	for i := 0; i < n; i++ {
		p.wr("\t")
	}
}

func (p pp) node(n Node, d int) {
	switch n := n.(type) {
	case *Module:
		p.wr("Module(%q)\n", n.File)
		for _, dcl := range n.Decls {
			p.node(dcl, d+1)
		}
	case *FuncDecl:
		p.tabs(d)
		p.wr("Func ")
		if n.Async {
			p.wr("async ")
		}
		p.wr("%s(", n.Name.Name)
		for i, pr := range n.Params {
			if i > 0 {
				p.wr(", ")
			}
			p.wr("%s", pr.Name.Name)
			if pr.Type != nil {
				p.wr(": %s", pr.Type.Name)
			}
			if pr.Default != nil {
				p.wr(" = ...")
			}
		}
		p.wr(")")
		if n.RetType != nil {
			p.wr(" -> %s", n.RetType.Name)
		}
		p.wr("\n")
		if n.Body != nil {
			p.node(n.Body, d+1)
		}
	case *Block:
		p.tabs(d)
		p.wr("Block\n")
		for _, s := range n.Stmts {
			p.node(s, d+1)
		}
	case *LetStmt:
		p.tabs(d)
		p.wr("Let ")
		if n.Mutable {
			p.wr("mut ")
		}
		p.wr("%s", n.Name.Name)
		if n.Type != nil {
			p.wr(": %s", n.Type.Name)
		}
		p.wr(" = ")
		p.node(n.Value, 0)
		p.wr("\n")
	case *ReturnStmt:
		p.tabs(d)
		p.wr("Return")
		if n.Value != nil {
			p.wr(" ")
			p.node(n.Value, 0)
		}
		p.wr("\n")
	case *ExprStmt:
		p.tabs(d)
		p.node(n.Expr, 0)
		p.wr("\n")
	case *Ident:
		p.wr("Ident(%s)", n.Name)
	case *IntLit:
		p.wr("Int(%s)", n.Text)
	case *FloatLit:
		p.wr("Float(%s)", n.Text)
	case *StrLit:
		p.wr("Str")
	case *BoolLit:
		if n.Value {
			p.wr("true")
		} else {
			p.wr("false")
		}
	case *NoneLit:
		p.wr("none")
	case *UnaryExpr:
		p.wr("(%s ", n.Op)
		p.node(n.X, 0)
		p.wr(")")
	case *BinaryExpr:
		p.wr("(")
		p.node(n.Lhs, 0)
		p.wr(" %s ", n.Op)
		p.node(n.Rhs, 0)
		p.wr(")")
	case *CallExpr:
		p.wr("Call ")
		p.node(n.Callee, 0)
		p.wr("(")
		for i, a := range n.Args {
			if i > 0 {
				p.wr(", ")
			}
			p.node(a, 0)
		}
		p.wr(")")
	case *IndexExpr:
		p.wr("Index ")
		p.node(n.X, 0)
		p.wr("[")
		p.node(n.Idx, 0)
		p.wr("]")
	case *FieldExpr:
		p.wr("Field ")
		p.node(n.X, 0)
		p.wr(".%s", n.Name.Name)
	default:
		p.tabs(d)
		p.wr("<?>%T\n", n)
	}
}

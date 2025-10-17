package ast

import (
	"fmt"
	"io"
)

// Print renders a stable, tab-indented AST suitable for -ast dumps and tests.
// It intentionally avoids source round-tripping; strings/ints are summarized.
func Print(w io.Writer, n Node) {
	pp{w: w}.node(n, 0)
}

type pp struct{ w io.Writer }

func (p pp) wr(f string, args ...any) { _, _ = fmt.Fprintf(p.w, f, args...) }
func (p pp) tabs(d int) {
	for i := 0; i < d; i++ {
		_, _ = io.WriteString(p.w, "\t")
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
		// decorators
		for _, dec := range n.Decorators {
			p.tabs(d)
			p.wr("@%s", dec.Name.Name)
			if len(dec.Args) > 0 {
				p.wr("(")
				for i, a := range dec.Args {
					if i > 0 {
						p.wr(", ")
					}
					p.node(a, 0)
				}
				p.wr(")")
			}
			p.wr("\n")
		}
		p.tabs(d)
		p.wr("Func ")
		if n.Async {
			p.wr("async ")
		}
		if n.Pub {
			p.wr("pub ")
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
		if n.Doc != nil {
			p.tabs(d + 1)
			p.wr("DocString\n")
		}
		if n.Body != nil {
			p.node(n.Body, d+1)
		}

	case *ClassDecl:
		// decorators
		for _, dec := range n.Decorators {
			p.tabs(d)
			p.wr("@%s", dec.Name.Name)
			if len(dec.Args) > 0 {
				p.wr("(")
				for i, a := range dec.Args {
					if i > 0 {
						p.wr(", ")
					}
					p.node(a, 0)
				}
				p.wr(")")
			}
			p.wr("\n")
		}
		p.tabs(d)
		p.wr("Class ")
		if n.Pub {
			p.wr("pub ")
		}
		p.wr("%s", n.Name.Name)
		if len(n.Bases) > 0 {
			p.wr("(")
			for i, b := range n.Bases {
				if i > 0 {
					p.wr(", ")
				}
				p.wr("%s", b.Name)
			}
			p.wr(")")
		}
		p.wr("\n")
		if n.Doc != nil {
			p.tabs(d + 1)
			p.wr("DocString\n")
		}
		for _, f := range n.Fields {
			p.tabs(d + 1)
			p.wr("Field ")
			if f.Pub {
				p.wr("pub ")
			}
			p.wr("%s", f.Name.Name)
			if f.Type != nil {
				p.wr(": %s", f.Type.Name)
			}
			p.wr("\n")
		}
		for _, c := range n.Nested {
			p.node(c, d+1)
		}
		for _, m := range n.Methods {
			p.node(m, d+1)
		}

	case *StructDecl:
		for _, dec := range n.Decorators {
			p.tabs(d)
			p.wr("@%s", dec.Name.Name)
			if len(dec.Args) > 0 {
				p.wr("(")
				for i, a := range dec.Args {
					if i > 0 {
						p.wr(", ")
					}
					p.node(a, 0)
				}
				p.wr(")")
			}
			p.wr("\n")
		}
		p.tabs(d)
		p.wr("Struct ")
		if n.Pub {
			p.wr("pub ")
		}
		p.wr("%s\n", n.Name.Name)
		if n.Doc != nil {
			p.tabs(d + 1)
			p.wr("DocString\n")
		}
		for _, f := range n.Fields {
			p.tabs(d + 1)
			p.wr("Field ")
			if f.Pub {
				p.wr("pub ")
			}
			p.wr("%s", f.Name.Name)
			if f.Type != nil {
				p.wr(": %s", f.Type.Name)
			}
			p.wr("\n")
		}

	case *EnumDecl:
		for _, dec := range n.Decorators {
			p.tabs(d)
			p.wr("@%s", dec.Name.Name)
			if len(dec.Args) > 0 {
				p.wr("(")
				for i, a := range dec.Args {
					if i > 0 {
						p.wr(", ")
					}
					p.node(a, 0)
				}
				p.wr(")")
			}
			p.wr("\n")
		}
		p.tabs(d)
		p.wr("Enum ")
		if n.Pub {
			p.wr("pub ")
		}
		p.wr("%s\n", n.Name.Name)
		if n.Doc != nil {
			p.tabs(d + 1)
			p.wr("DocString\n")
		}
		for _, v := range n.Variants {
			p.tabs(d + 1)
			p.wr("Variant %s", v.Name.Name)
			if v.Type != nil {
				p.wr(": %s", v.Type.Name)
			} else {
				p.wr(": none")
			}
			p.wr("\n")
		}

	case *Block:
		p.tabs(d)
		p.wr("Block\n")
		for _, s := range n.Stmts {
			p.node(s, d+1)
		}

	case *IfStmt:
		p.tabs(d)
		p.wr("If ")
		p.node(n.Cond, 0)
		p.wr("\n")
		if n.Then != nil {
			p.node(n.Then, d+1)
		}
		for _, arm := range n.Elifs {
			p.tabs(d)
			p.wr("Elif ")
			p.node(arm.Cond, 0)
			p.wr("\n")
			if arm.Body != nil {
				p.node(arm.Body, d+1)
			}
		}
		if n.Else != nil {
			p.tabs(d)
			p.wr("Else\n")
			p.node(n.Else, d+1)
		}

	case *WhileStmt:
		p.tabs(d)
		p.wr("While ")
		p.node(n.Cond, 0)
		p.wr("\n")
		if n.Body != nil {
			p.node(n.Body, d+1)
		}

	case *ForStmt:
		p.tabs(d)
		p.wr("For ")
		p.node(n.Target, 0)
		p.wr(" in ")
		p.node(n.Iter, 0)
		p.wr("\n")
		if n.Body != nil {
			p.node(n.Body, d+1)
		}

	case *UsingStmt:
		p.tabs(d)
		p.wr("Using ")
		if n.Bind != nil {
			p.node(n.Bind, 0)
		}
		if n.Init != nil {
			p.wr(" = ")
			p.node(n.Init, 0)
		}
		p.wr("\n")
		if n.Body != nil {
			p.node(n.Body, d+1)
		}

	case *DeferStmt:
		p.tabs(d)
		p.wr("Defer ")
		if n.Call != nil {
			p.node(n.Call, 0)
		} else {
			p.wr("<call>")
		}
		p.wr("\n")

	case *DocStringStmt:
		p.tabs(d)
		p.wr("DocString\n")

	// --- Expressions ---
	case *Ident:
		p.wr("%s", n.Name)

	case *IntLit:
		p.wr("%s", n.Text)

	case *FloatLit:
		p.wr("%s", n.Text)

	case *StrLit:
		if n.Long {
			p.wr(`"""..."""`)
		} else {
			p.wr(`"..."`)
		}

	case *UnaryExpr:
		p.wr("(%s", n.Op)
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

	case *FieldExpr:
		p.node(n.X, 0)
		p.wr(".%s", n.Name.Name)

	case *IndexExpr:
		p.node(n.X, 0)
		p.wr("[")
		p.node(n.Idx, 0)
		p.wr("]")

	case *AssignStmt:
		p.tabs(d)
		p.wr("Assign\n")
		for i, t := range n.LHS {
			p.tabs(d + 1)
			p.wr("LHS[%d] ", i)
			p.node(t, 0)
			p.wr("\n")
		}
		for i, v := range n.RHS {
			p.tabs(d + 1)
			p.wr("RHS[%d] ", i)
			p.node(v, 0)
			p.wr("\n")
		}

	case *AugAssignStmt:
		p.tabs(d)
		p.wr("AugAssign %s ", n.Op)
		p.node(n.Left, 0)
		p.wr(" ")
		p.node(n.Right, 0)
		p.wr("\n")

	default:
		p.tabs(d)
		p.wr("<%T>\n", n)
	}
}

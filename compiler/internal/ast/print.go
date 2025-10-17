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

	/* ---------- Module / Decls ---------- */

	case *Module:
		p.wr("Module(%q)\n", n.File)
		for _, dcl := range n.Decls {
			p.node(dcl, d+1)
		}

	case *FuncDecl:
		// decorators first
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
				// tests only care that a default exists, not its value
				p.wr(" = .")
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
		// fields
		for _, f := range n.Fields {
			p.tabs(d + 1)
			p.wr("Field ")
			if f.Pub {
				p.wr("pub ")
			}
			p.wr("%s: ", f.Name.Name)
			if f.Type != nil {
				p.wr("%s", f.Type.Name)
			} else {
				p.wr("?")
			}
			p.wr("\n")
		}
		// nested classes
		for _, c := range n.Nested {
			p.node(c, d+1)
		}
		// methods
		for _, m := range n.Methods {
			p.node(m, d+1)
		}

	case *StructDecl:
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
			p.wr("%s: ", f.Name.Name)
			if f.Type != nil {
				p.wr("%s\n", f.Type.Name)
			} else {
				p.wr("?\n")
			}
		}

	case *EnumDecl:
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
			p.wr("%s: ", v.Name.Name)
			if v.Type != nil {
				p.wr("%s\n", v.Type.Name)
			} else {
				p.wr("none\n")
			}
		}

	/* ---------- Blocks & Statements ---------- */

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

	case *AssignStmt:
		p.tabs(d)
		for i, e := range n.LHS {
			if i > 0 {
				p.wr(", ")
			}
			p.node(e, 0)
		}
		p.wr(" := ")
		for i, e := range n.RHS {
			if i > 0 {
				p.wr(", ")
			}
			p.node(e, 0)
		}
		p.wr("\n")

	case *AugAssignStmt:
		p.tabs(d)
		p.node(n.Left, 0)
		p.wr(" %s ", n.Op)
		p.node(n.Right, 0)
		p.wr("\n")

	case *ReturnStmt:
		p.tabs(d)
		p.wr("Return")
		if n.Value != nil {
			p.wr(" ")
			p.node(n.Value, 0)
		}
		p.wr("\n")

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
			p.node(arm.Body, d+1)
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
		p.node(n.Bind, 0)
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
			p.wr("<invalid>")
		}
		p.wr("\n")

	case *DocStringStmt:
		p.tabs(d)
		p.wr("DocString\n")

	/* ---------- Expressions ---------- */

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

	// --- NEW: Lambda & Comprehensions ---

	case *LambdaExpr:
		p.wr("Lambda(")
		for i, prm := range n.Params {
			if i > 0 {
				p.wr(", ")
			}
			p.wr("%s", prm.Name.Name)
			if prm.Type != nil {
				p.wr(": %s", prm.Type.Name)
			}
		}
		p.wr(") => ")
		p.node(n.Body, 0)

	case *ListComp:
		p.wr("[")
		p.node(n.Elem, 0)
		for i, c := range n.Clauses {
			_ = i
			p.wr(" for ")
			p.node(c.Target, 0)
			p.wr(" in ")
			p.node(c.Iter, 0)
			if c.If != nil {
				p.wr(" if ")
				p.node(c.If, 0)
			}
		}
		p.wr("]")

	case *DictComp:
		p.wr("{")
		p.node(n.Key, 0)
		p.wr(": ")
		p.node(n.Val, 0)
		for i, c := range n.Clauses {
			_ = i
			p.wr(" for ")
			p.node(c.Target, 0)
			p.wr(" in ")
			p.node(c.Iter, 0)
			if c.If != nil {
				p.wr(" if ")
				p.node(c.If, 0)
			}
		}
		p.wr("}")

	case *SetComp:
		p.wr("#{")
		p.node(n.Elem, 0)
		for i, c := range n.Clauses {
			_ = i
			p.wr(" for ")
			p.node(c.Target, 0)
			p.wr(" in ")
			p.node(c.Iter, 0)
			if c.If != nil {
				p.wr(" if ")
				p.node(c.If, 0)
			}
		}
		p.wr("}")

	default:
		p.tabs(d)
		p.wr("<?>%T\n", n)
	}
}

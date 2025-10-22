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
			p.tabs(d + 1)
			p.node(dcl, d+1)
		}

	case *FuncDecl:
		p.tabs(d)
		p.wr("Func %s(", n.Name.Name)
		for i, pr := range n.Params {
			if i > 0 {
				p.wr(", ")
			}
			p.wr("%s", pr.Name.Name)
			if pr.Type != nil {
				p.wr(": %s", pr.Type.Name)
			}
			if pr.Default != nil {
				p.wr(" = .")
			}
		}
		p.wr(")")
		if n.RetType != nil {
			p.wr(" -> %s", n.RetType.Name)
		}
		p.wr("\n")
		if len(n.Decorators) > 0 {
			for _, dec := range n.Decorators {
				p.tabs(d + 1)
				p.node(dec, d+1)
			}
		}
		if n.Doc != nil {
			p.tabs(d + 1)
			p.node(&DocStringStmt{Text: n.Doc}, d+1)
		}
		if n.Body != nil {
			p.node(n.Body, d+1)
		}

	case *Decorator:
		p.wr("@%s", n.Name.Name)
		if len(n.Args) > 0 {
			p.wr("(")
			for i, a := range n.Args {
				if i > 0 {
					p.wr(", ")
				}
				p.node(a, 0)
			}
			p.wr(")")
		}
		p.wr("\n")

	case *StructDecl:
		p.tabs(d)
		if n.Pub {
			p.wr("pub ")
		}
		p.wr("Struct %s\n", n.Name.Name)
		if len(n.Decorators) > 0 {
			for _, dec := range n.Decorators {
				p.tabs(d + 1)
				p.node(dec, d+1)
			}
		}
		if n.Doc != nil {
			p.tabs(d + 1)
			p.node(&DocStringStmt{Text: n.Doc}, d+1)
		}
		for _, f := range n.Fields {
			p.tabs(d + 1)
			if f.Pub {
				p.wr("pub ")
			}
			p.wr("%s: %s\n", f.Name.Name, f.Type.Name)
		}

	case *EnumDecl:
		p.tabs(d)
		if n.Pub {
			p.wr("pub ")
		}
		p.wr("Enum %s\n", n.Name.Name)
		if len(n.Decorators) > 0 {
			for _, dec := range n.Decorators {
				p.tabs(d + 1)
				p.node(dec, d+1)
			}
		}
		if n.Doc != nil {
			p.tabs(d + 1)
			p.node(&DocStringStmt{Text: n.Doc}, d+1)
		}
		for _, v := range n.Variants {
			p.tabs(d + 1)
			p.wr("%s: ", v.Name.Name)
			if v.Type != nil {
				p.wr("%s", v.Type.Name)
			} else {
				p.wr("none")
			}
			p.wr("\n")
		}

	case *ClassDecl:
		p.tabs(d)
		if n.Pub {
			p.wr("pub ")
		}
		p.wr("Class %s", n.Name.Name)
		if len(n.Bases) > 0 {
			p.wr(" (")
			for i, b := range n.Bases {
				if i > 0 {
					p.wr(", ")
				}
				p.wr("%s", b.Name)
			}
			p.wr(")")
		}
		p.wr("\n")
		if len(n.Decorators) > 0 {
			for _, dec := range n.Decorators {
				p.tabs(d + 1)
				p.node(dec, d+1)
			}
		}
		if n.Doc != nil {
			p.tabs(d + 1)
			p.node(&DocStringStmt{Text: n.Doc}, d+1)
		}
		for _, f := range n.Fields {
			p.tabs(d + 1)
			if f.Pub {
				p.wr("pub ")
			}
			p.wr("%s: %s\n", f.Name.Name, f.Type.Name)
		}
		for _, sub := range n.Nested {
			p.node(sub, d+1)
		}
		for _, m := range n.Methods {
			p.node(m, d+1)
		}

	case *DocStringStmt:
		p.tabs(d)
		p.wr(`Doc("""...""")` + "\n")

	case *Block:
		p.tabs(d)
		p.wr("Block\n")
		for _, s := range n.Stmts {
			p.node(s, d+1)
		}

	case *LetStmt:
		p.tabs(d)
		p.wr("Let %s = ", n.Name.Name)
		p.node(n.Init, 0)
		p.wr("\n")

	case *AssignStmt:
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

	// >>> NEW: Match <<<
	case *MatchStmt:
		p.tabs(d)
		p.wr("Match ")
		p.node(n.Scrutinee, 0)
		p.wr("\n")
		for _, arm := range n.Arms {
			p.tabs(d)
			p.wr("Case ")
			if arm.Pattern != nil {
				p.node(arm.Pattern, 0)
			}
			p.wr(": ")
			if arm.Result != nil {
				p.node(arm.Result, 0)
			}
			p.wr("\n")
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
		}
		p.wr("\n")

	case *UsingStmt:
		p.tabs(d)
		p.wr("Using ")
		p.node(n.Resource, 0)
		p.wr("\n")
		if n.Body != nil {
			p.node(n.Body, d+1)
		}

	/* ---------- Exprs ---------- */

	case *BinExpr:
		p.wr("(")
		p.node(n.Left, 0)
		p.wr(" %s ", n.Op)
		p.node(n.Right, 0)
		p.wr(")")

	case *UnaryExpr:
		p.wr("%s ", n.Op)
		p.node(n.Expr, 0)

	case Ident:
		p.wr("Ident(%s)", n.Name)

	case *IntLit:
		p.wr("Int(%d)", n.Value)

	case *FloatLit:
		p.wr("Float(.)")

	case *StrLit:
		if n.Long {
			p.wr(`Str("""...""")`)
		} else {
			p.wr(`Str("...")`)
		}

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
		p.node(n.Target, 0)
		p.wr("[")
		p.node(n.Index, 0)
		p.wr("]")

	case *FieldExpr:
		p.wr("Field ")
		p.node(n.Target, 0)
		p.wr(".%s", n.Name.Name)

	case *LambdaExpr:
		p.wr("Lambda(")
		for i, pr := range n.Params {
			if i > 0 {
				p.wr(", ")
			}
			p.wr("%s", pr.Name.Name)
			if pr.Type != nil {
				p.wr(": %s", pr.Type.Name)
			}
		}
		p.wr(") => ")
		if n.Body != nil {
			p.node(n.Body, 0)
		}

	case *ListCompExpr:
		p.wr("[")
		p.node(n.Elem, 0)
		for _, c := range n.Clauses {
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

	case *DictCompExpr:
		p.wr("{")
		p.node(n.Key, 0)
		p.wr(": ")
		p.node(n.Value, 0)
		for _, c := range n.Clauses {
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

	case *SetCompExpr:
		p.wr("#{")
		p.node(n.Elem, 0)
		for _, c := range n.Clauses {
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
		p.wr("<?>%T", n)
		p.wr("\n")
	}
}

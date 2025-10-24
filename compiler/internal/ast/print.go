package ast

import (
	"fmt"
	"io"
	"strings"
)

// Print renders a stable, tab-indented AST suitable for -ast dumps and tests.
// It intentionally avoids source round-tripping; numbers are normalized;
// short strings are summarized as Str("...") except where tests require
// real text (match arm results).
func Print(w io.Writer, n Node) {
	pp{w: w}.node(n, 0)
}

type pp struct {
	w              io.Writer
	showStrLiteral bool // when true, print short string literal contents
}

func (p pp) wr(f string, args ...any) { _, _ = fmt.Fprintf(p.w, f, args...) }

func (p pp) tabs(d int) {
	for i := 0; i < d; i++ {
		_, _ = io.WriteString(p.w, "\t")
	}
}

func (p pp) withStrValues(on bool) pp { return pp{w: p.w, showStrLiteral: on} }

func (p pp) node(n Node, d int) {
	switch n := n.(type) {

	/* ---------- Module / Decls ---------- */

	case *Module:
		p.wr("Module(%q)\n", n.File)
		for _, dcl := range n.Decls {
			// Children indent themselves.
			p.node(dcl, d+1)
		}

	case *FuncDecl:
		// Decorators first, at the same indent as the header.
		for _, dec := range n.Decorators {
			p.node(dec, d)
		}
		p.tabs(d)
		if n.Async {
			p.wr("Async ")
		}
		p.wr("Func ")
		if n.Pub {
			p.wr("pub ")
		}
		p.wr("%s(", n.Name.Name)
		for i, p0 := range n.Params {
			if i > 0 {
				p.wr(", ")
			}
			p.wr("%s", p0.Name.Name)
			if p0.Type != nil {
				p.wr(": %s", typeNameStr(p0.Type))
			}
			if p0.Default != nil {
				p.wr(" = ")
				p.node(p0.Default, 0)
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
		for _, dec := range n.Decorators {
			p.node(dec, d)
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
				p.node(b, 0)
			}
			p.wr(")")
		}
		p.wr("\n")
		if n.Doc != nil {
			p.tabs(d + 1)
			p.wr("DocString\n")
		}
		for _, f := range n.Fields {
			p.node(f, d+1)
		}
		for _, m0 := range n.Methods {
			p.node(m0, d+1)
		}
		for _, c0 := range n.Nested {
			p.node(c0, d+1)
		}

	case *StructDecl:
		for _, dec := range n.Decorators {
			p.node(dec, d)
		}
		p.tabs(d)
		p.wr("Struct ")
		if n.Pub {
			p.wr("pub ")
		}
		p.wr("%s", n.Name.Name)
		p.wr("\n")
		if n.Doc != nil {
			p.tabs(d + 1)
			p.wr("DocString\n")
		}
		for _, f := range n.Fields {
			p.node(f, d+1)
		}

	case *EnumDecl:
		for _, dec := range n.Decorators {
			p.node(dec, d)
		}
		p.tabs(d)
		p.wr("Enum ")
		if n.Pub {
			p.wr("pub ")
		}
		p.wr("%s", n.Name.Name)
		p.wr("\n")
		if n.Doc != nil {
			p.tabs(d + 1)
			p.wr("DocString\n")
		}
		for _, it := range n.Items {
			p.node(it, d+1)
		}

	case *Decorator:
		p.tabs(d)
		p.wr("@%s\n", n.Path)

	case *FieldDecl:
		p.tabs(d)
		p.wr("Field ")
		if n.Pub {
			p.wr("pub ")
		}
		p.wr("%s: %s\n", n.Name.Name, typeNameStr(n.Type))

	case *EnumItem:
		p.tabs(d)
		p.wr("EnumItem %s\n", n.Name.Name)

	/* ---------- Statements ---------- */

	case *Block:
		p.tabs(d)
		p.wr("Block\n")
		for _, s := range n.Stmts {
			p.node(s, d+1)
		}

	case *DocStringStmt:
		p.tabs(d)
		p.wr("DocString\n")

	case *LetStmt:
		p.tabs(d)
		p.wr("Let %s = ", n.Name.Name)
		if n.Value != nil {
			p.node(n.Value, 0)
		}
		p.wr("\n")

	case *AssignStmt:
		p.tabs(d)
		for i, x := range n.LHS {
			if i > 0 {
				p.wr(", ")
			}
			p.node(x, 0)
		}
		p.wr(" := ")
		for i, x := range n.RHS {
			if i > 0 {
				p.wr(", ")
			}
			p.node(x, 0)
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
		p.wr("Return ")
		if n.Value != nil {
			p.node(n.Value, 0)
		}
		p.wr("\n")

	case *ExprStmt:
		p.tabs(d)
		p.node(n.Expr, 0)
		p.wr("\n")

	case *IfStmt:
		p.tabs(d)
		p.wr("If ")
		p.node(n.Cond, 0)
		p.wr("\n")
		p.node(n.Then, d+1)
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
		p.node(n.Body, d+1)

	case *ForStmt:
		p.tabs(d)
		p.wr("For ")
		p.node(n.Target, 0)
		p.wr(" in ")
		p.node(n.Iter, 0)
		p.wr("\n")
		p.node(n.Body, d+1)

	case *UsingStmt:
		p.tabs(d)
		p.wr("Using ")
		p.node(n.Expr, 0)
		p.wr("\n")
		p.node(n.Body, d+1)

	case *DeferStmt:
		p.tabs(d)
		p.wr("Defer ")
		p.node(n.Call, 0)
		p.wr("\n")

	case *MatchStmt:
		p.tabs(d)
		p.wr("Match ")
		p.node(n.Value, 0)
		p.wr("\n")
		for _, arm := range n.Arms {
			p.node(arm, d+1)
		}

	case *MatchArm:
		p.tabs(d)
		p.wr("Arm ")
		p.node(n.Pat, 0)
		if n.Guard != nil {
			p.wr(" if ")
			p.node(n.Guard, 0)
		}
		p.wr(" -> ")
		p.withStrValues(true).node(n.Result, 0)
		p.wr("\n")

	/* ---------- IMPORTS (NEW) ---------- */

	case *ImportStmt:
		p.tabs(d)
		p.wr("Import %s", strings.Join(n.Path, "."))
		if n.Alias != nil {
			p.wr(" as %s", n.Alias.Name)
		}
		p.wr("\n")

	case *FromImportStmt:
		p.tabs(d)
		p.wr("From %s import ", strings.Join(n.Path, "."))
		for i, it := range n.Items {
			if i > 0 {
				p.wr(", ")
			}
			p.wr("%s", it.Name.Name)
			if it.Alias != nil {
				p.wr(" as %s", it.Alias.Name)
			}
		}
		p.wr("\n")

	/* ---------- Expressions ---------- */

	case *Ident:
		p.wr("%s", n.Name)

	case *IntLit:
		p.wr("Int(%s)", n.Text)

	case *FloatLit:
		p.wr("Float(%s)", n.Text)

	case *StrLit:
		if n.Long {
			p.wr(`Str("""...""")`)
			return
		}
		if p.showStrLiteral {
			p.wr(`Str("...")`)
		} else {
			p.wr("Str")
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
		p.node(n.Seq, 0)
		p.wr("[")
		p.node(n.Index, 0)
		p.wr("]")

	case *FieldExpr:
		p.node(n.Recv, 0)
		p.wr(".%s", n.Name.Name)

	case *UnaryExpr:
		p.wr("%s ", n.Op)
		p.node(n.Expr, 0)

	case *BinaryExpr:
		p.node(n.Left, 0)
		p.wr(" %s ", n.Op)
		p.node(n.Right, 0)

	case *LambdaExpr:
		p.wr("Lambda(")
		for i, a := range n.Params {
			if i > 0 {
				p.wr(", ")
			}
			p.wr("%s", a.Name.Name)
			if a.Type != nil {
				p.wr(": %s", typeNameStr(a.Type))
			}
		}
		p.wr(") => ")
		p.node(n.Body, 0)

	case *CompExpr:
		p.wr("Comp ")
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
		p.tabs(d)
		p.wr("<?>%T\n", n)
	}
}

func typeNameStr(t *TypeName) string {
	if t == nil {
		return ""
	}
	return t.Name
}

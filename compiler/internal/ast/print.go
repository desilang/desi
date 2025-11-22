package ast

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Print pretty-prints an AST node in the compact, test-friendly format used
// throughout the repository.
func Print(w io.Writer, n Node) { (&pp{w: w}).node(n, 0) }

// internal pretty-printer
type pp struct {
	w              io.Writer
	showStrLiteral bool // when true, render Str("...") instead of just Str
}

func (p *pp) withStrValues(on bool) *pp {
	q := *p
	q.showStrLiteral = on
	return &q
}

func (p *pp) wr(f string, a ...any) { _, _ = fmt.Fprintf(p.w, f, a...) }
func (p *pp) tabs(n int) {
	for i := 0; i < n; i++ {
		p.wr("\t")
	}
}

func typeNameStr(t *TypeName) string {
	if t == nil {
		return ""
	}
	return t.Name
}

func (p *pp) printParams(params []Param) {
	for i, prm := range params {
		if i > 0 {
			p.wr(", ")
		}
		// Print parameter mode prefix if present.
		switch prm.Mode {
		case ParamRef:
			p.wr("ref ")
		case ParamInout:
			p.wr("inout ")
		default:
		}
		p.wr("%s", prm.Name.Name)
		if prm.Type != nil {
			p.wr(": %s", typeNameStr(prm.Type))
		}
		if prm.Default != nil {
			p.wr(" = ")
			p.node(prm.Default, 0)
		}
	}
}

/* ---------------------------- main dispatcher ---------------------------- */

func (p *pp) node(n Node, d int) {
	switch n := n.(type) {

	/* ---------- Module / Decls ---------- */

	case *Module:
		p.wr("Module(%q)\n", n.File)
		for _, dec := range n.Decls {
			p.node(dec, 1)
		}

	case *Decorator:
		p.tabs(d)
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

	case *FuncDecl:
		// decorators first
		for _, dec := range n.Decorators {
			p.node(dec, d)
		}
		p.tabs(d)
		p.wr("Func ")
		if n.Pub {
			p.wr("pub ")
		}
		if n.Async {
			p.wr("async ")
		}
		p.wr("%s(", n.Name.Name)
		p.printParams(n.Params)
		p.wr(")")
		if n.RetType != nil {
			p.wr(" -> %s", typeNameStr(n.RetType))
		}
		p.wr("\n")
		if n.Doc != nil {
			p.tabs(d + 1)
			p.wr("DocString\n")
		}
		if n.Body != nil {
			p.node(n.Body, d+1)
		}

	case *FieldDecl:
		p.tabs(d)
		p.wr("Field ")
		if n.Pub {
			p.wr("pub ")
		}
		p.wr("%s", n.Name.Name)
		if n.Type != nil {
			p.wr(": %s", typeNameStr(n.Type))
		}
		p.wr("\n")

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
				p.wr("%s", typeNameStr(b))
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
			p.node(f, d+1)
		}
		// nested
		for _, c := range n.Nested {
			p.node(c, d+1)
		}
		// methods
		for _, m := range n.Methods {
			p.node(m, d+1)
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
		p.wr("%s\n", n.Name.Name)
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
		p.wr("%s\n", n.Name.Name)
		if n.Doc != nil {
			p.tabs(d + 1)
			p.wr("DocString\n")
		}
		for _, v := range n.Variants {
			p.tabs(d + 1)
			p.wr("Variant %s", v.Name.Name)
			if v.Type != nil {
				p.wr(": %s", typeNameStr(v.Type))
			}
			p.wr("\n")
		}

	/* ---------- Blocks ---------- */

	case *Block:
		p.tabs(d)
		p.wr("Block\n")
		for _, s := range n.Stmts {
			p.node(s, d+1)
		}

	/* ---------- Statements ---------- */

	case *DocStringStmt:
		p.tabs(d)
		p.wr("DocString\n")

	case *LetStmt:
		p.tabs(d)
		p.wr("Let %s", n.Name.Name)
		if n.Type != nil {
			p.wr(": %s", typeNameStr(n.Type))
		}
		p.wr(" = ")
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
		p.wr(" :=")
		p.wr(" ")
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

	// Parse-only Match (value arms)
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
				// For tests, show actual short string in match arm result.
				p.withStrValues(true).node(arm.Result, 0)
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

	// NEW (M9C): unsafe block
	case *UnsafeBlock:
		p.tabs(d)
		p.wr("Unsafe\n")
		if n.Body != nil {
			p.node(n.Body, d+1)
		}

	// NEW (M5): imports
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

	/* ---------- Exprs ---------- */

	case *Ident:
		p.wr("Ident(%s)", n.Name)

	case *IntLit:
		// normalize underscores / base for stabilized printing
		txt := n.Text
		if strings.HasPrefix(txt, "0x") || strings.HasPrefix(txt, "0X") {
			if v, err := strconv.ParseUint(txt[2:], 16, 64); err == nil {
				txt = strconv.FormatUint(v, 10)
			}
		}
		txt = strings.ReplaceAll(txt, "_", "")
		p.wr("Int(%s)", txt)

	case *FloatLit:
		p.wr("Float(%s)", strings.ReplaceAll(n.Text, "_", ""))

	case *StrLit:
		if p.showStrLiteral {
			p.wr(`Str("...")`)
		} else {
			p.wr("Str")
		}

	case *FString:
		p.wr("FString(")
		for i, part := range n.Parts {
			if i > 0 {
				p.wr(", ")
			}
			p.node(part, 0)
		}
		p.wr(")")

	case *BoolLit:
		if n.Value {
			p.wr("True")
		} else {
			p.wr("False")
		}

	case *NoneLit:
		p.wr("None")

	case *DictLit:
		p.wr("{")
		for i := range n.Keys {
			if i > 0 {
				p.wr(", ")
			}
			p.node(n.Keys[i], 0)
			p.wr(": ")
			p.node(n.Values[i], 0)
		}
		p.wr("}")

	case *UnaryExpr:
		p.wr("( %s ", n.Op)
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
		if len(n.ArgNodes) > 0 {
			for i, a := range n.ArgNodes {
				if i > 0 {
					p.wr(", ")
				}
				if a.Star {
					p.wr("*")
				}
				if a.Name != nil {
					p.wr("%s=", a.Name.Name)
				}
				if a.Expr != nil {
					p.node(a.Expr, 0)
				}
			}
		} else {
			for i, a := range n.Args {
				if i > 0 {
					p.wr(", ")
				}
				p.node(a, 0)
			}
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

	case *LambdaExpr:
		p.wr("Lambda(")
		for i, prm := range n.Params {
			if i > 0 {
				p.wr(", ")
			}
			p.wr("%s", prm.Name.Name)
			if prm.Type != nil {
				p.wr(": %s", typeNameStr(prm.Type))
			}
		}
		p.wr(") => ")
		p.node(n.Body, 0)

	case *ListComp:
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

	case *DictComp:
		p.wr("{")
		p.node(n.Key, 0)
		p.wr(": ")
		p.node(n.Val, 0)
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

	case *SetComp:
		p.wr("#{")
		p.node(n.Elem, 0)
		for _, cl := range n.Clauses {
			p.wr(" for ")
			p.node(cl.Target, 0)
			p.wr(" in ")
			p.node(cl.Iter, 0)
			if cl.If != nil {
				p.wr(" if ")
				p.node(cl.If, 0)
			}
		}
		p.wr("}")

	default:
		p.wr("<unknown %T>", n)
	}
}

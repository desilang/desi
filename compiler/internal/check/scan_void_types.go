package check

import (
	"fmt"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
)

// ---- NEW: scan textual 'void' in type positions ----

func scanForVoidTypes(f *ast.File) []error {
	var out []error

	isVoid := func(s string) bool {
		return strings.EqualFold(strings.TrimSpace(s), "void")
	}
	hasSpan := func(sp ast.Span) bool { return sp != (ast.Span{}) }

	spanOfStmt := func(s ast.Stmt) ast.Span {
		switch t := s.(type) {
		case *ast.LetStmt:
			return t.Span
		case *ast.AssignStmt:
			return t.Span
		case *ast.ReturnStmt:
			return t.Span
		case *ast.ExprStmt:
			return t.Span
		case *ast.IfStmt:
			return t.Span
		case *ast.WhileStmt:
			return t.Span
		case *ast.DeferStmt:
			return t.Span
		case *ast.MatchStmt:
			return t.Span
		default:
			return ast.Span{}
		}
	}

	bestSpanForFunc := func(fd *ast.FuncDecl) ast.Span {
		// Prefer the function decl span if present.
		if hasSpan(fd.Span) {
			return fd.Span
		}
		// Then any parameter span.
		for _, p := range fd.Params {
			if hasSpan(p.Span) {
				return p.Span
			}
		}
		// Then the first stmt span in the body.
		if len(fd.Body) > 0 {
			if sp := spanOfStmt(fd.Body[0]); hasSpan(sp) {
				return sp
			}
		}
		return ast.Span{}
	}

	add := func(where, typ string, sp ast.Span) {
		if isVoid(typ) {
			if hasSpan(sp) {
				out = append(out, ErrUseNoneInsteadOfVoidAt(sp, where))
			} else {
				out = append(out, ErrUseNoneInsteadOfVoid(where))
			}
		}
	}

	for _, d := range f.Decls {
		switch v := d.(type) {
		case *ast.FuncDecl:
			add(fmt.Sprintf("function %q return type", v.Name), v.Ret, bestSpanForFunc(v))
			for _, p := range v.Params {
				add(fmt.Sprintf("parameter %q of function %q", p.Name, v.Name), p.Type, p.Span)
			}
		case *ast.StructDecl:
			for _, ft := range v.Fields {
				add(fmt.Sprintf("field %q of struct %q", ft.Name, v.Name), ft.Type, ft.Span)
			}
		case *ast.EnumDecl:
			for _, ev := range v.Variants {
				add(fmt.Sprintf("payload of %s.%s", v.Name, ev.Name), ev.Payload, ev.Span)
			}
		case *ast.TypeDecl:
			add(fmt.Sprintf("type alias %q", v.Name), v.Underlying, v.Span)
		}
	}
	return out
}

package lower

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
)

// Enforce: no unique ('inout') borrows live across an 'await' suspension point.
// Tier-0 conservative rule: if an async function has any inout param and we see
// any use of that param after the first await, emit DBR0001 and abort lowering.
func CheckAwaitBorrowBarrier(fd *ast.FuncDecl) []diag.Diagnostic {
	if fd == nil || fd.Body == nil || !fd.Async {
		return nil
	}

	// Gather inout param names.
	inout := map[string]ast.Ident{}
	for _, p := range fd.Params {
		if p.Mode == ast.ParamInout {
			inout[p.Name.Name] = p.Name
		}
	}
	if len(inout) == 0 {
		return nil
	}

	// ---- helpers ------------------------------------------------------------

	// Find first 'await' span in an expression (if any).
	var awaitSpanInExpr func(e ast.Expr) (diag.Span, bool)
	awaitSpanInExpr = func(e ast.Expr) (diag.Span, bool) {
		if e == nil {
			return diag.Span{}, false
		}
		switch x := e.(type) {
		case *ast.UnaryExpr:
			if x.Op == "await" {
				return x.SpanOf(), true
			}
			return awaitSpanInExpr(x.X)
		case *ast.CallExpr:
			if sp, ok := awaitSpanInExpr(x.Callee); ok {
				return sp, true
			}
			for _, a := range x.Args {
				if sp, ok := awaitSpanInExpr(a); ok {
					return sp, true
				}
			}
			return diag.Span{}, false
		case *ast.BinaryExpr:
			if sp, ok := awaitSpanInExpr(x.Lhs); ok {
				return sp, true
			}
			return awaitSpanInExpr(x.Rhs)
		default:
			return diag.Span{}, false
		}
	}

	// Does this expr reference any of the given names? If so, return its span + name.
	var usesInoutInExpr func(e ast.Expr) (bool, diag.Span, string)
	usesInoutInExpr = func(e ast.Expr) (bool, diag.Span, string) {
		if e == nil {
			return false, diag.Span{}, ""
		}
		switch x := e.(type) {
		case *ast.Ident:
			if _, ok := inout[x.Name]; ok {
				return true, x.Span, x.Name
			}
			return false, diag.Span{}, ""
		case *ast.UnaryExpr:
			return usesInoutInExpr(x.X)
		case *ast.CallExpr:
			if hit, sp, nm := usesInoutInExpr(x.Callee); hit {
				return true, sp, nm
			}
			for _, a := range x.Args {
				if hit, sp, nm := usesInoutInExpr(a); hit {
					return true, sp, nm
				}
			}
			return false, diag.Span{}, ""
		case *ast.BinaryExpr:
			if hit, sp, nm := usesInoutInExpr(x.Lhs); hit {
				return true, sp, nm
			}
			return usesInoutInExpr(x.Rhs)
		default:
			return false, diag.Span{}, ""
		}
	}

	// Does this statement contain an await? Also returns the first await span.
	var awaitSpanInStmt func(s ast.Stmt) (diag.Span, bool)
	awaitSpanInStmt = func(s ast.Stmt) (diag.Span, bool) {
		switch st := s.(type) {
		case *ast.AssignStmt:
			for _, e := range st.RHS {
				if sp, ok := awaitSpanInExpr(e); ok {
					return sp, true
				}
			}
			return diag.Span{}, false
		case *ast.ExprStmt:
			return awaitSpanInExpr(st.Expr)
		case *ast.IfStmt:
			return awaitSpanInExpr(st.Cond)
		case *ast.WhileStmt:
			return awaitSpanInExpr(st.Cond)
		case *ast.UsingStmt:
			if sp, ok := awaitSpanInExpr(st.Init); ok {
				return sp, true
			}
			return awaitSpanInExpr(st.Bind)
		default:
			return diag.Span{}, false
		}
	}

	// After we've crossed an await, find the first use of any inout param in a stmt.
	var usesInoutInStmt func(s ast.Stmt) (bool, diag.Span, string)
	usesInoutInStmt = func(s ast.Stmt) (bool, diag.Span, string) {
		switch st := s.(type) {
		case *ast.AssignStmt:
			for _, e := range st.LHS {
				if hit, sp, nm := usesInoutInExpr(e); hit {
					return true, sp, nm
				}
			}
			for _, e := range st.RHS {
				if hit, sp, nm := usesInoutInExpr(e); hit {
					return true, sp, nm
				}
			}
			return false, diag.Span{}, ""
		case *ast.ExprStmt:
			return usesInoutInExpr(st.Expr)
		case *ast.ReturnStmt:
			return usesInoutInExpr(st.Value)
		case *ast.IfStmt:
			if hit, sp, nm := usesInoutInExpr(st.Cond); hit {
				return true, sp, nm
			}
			if st.Then != nil {
				for _, ss := range st.Then.Stmts {
					if hit, sp, nm := usesInoutInStmt(ss); hit {
						return true, sp, nm
					}
				}
			}
			for _, e := range st.Elifs {
				if hit, sp, nm := usesInoutInExpr(e.Cond); hit {
					return true, sp, nm
				}
				if e.Body != nil {
					for _, ss := range e.Body.Stmts {
						if hit, sp, nm := usesInoutInStmt(ss); hit {
							return true, sp, nm
						}
					}
				}
			}
			if st.Else != nil {
				for _, ss := range st.Else.Stmts {
					if hit, sp, nm := usesInoutInStmt(ss); hit {
						return true, sp, nm
					}
				}
			}
			return false, diag.Span{}, ""
		case *ast.WhileStmt:
			if hit, sp, nm := usesInoutInExpr(st.Cond); hit {
				return true, sp, nm
			}
			if st.Body != nil {
				for _, ss := range st.Body.Stmts {
					if hit, sp, nm := usesInoutInStmt(ss); hit {
						return true, sp, nm
					}
				}
			}
			return false, diag.Span{}, ""
		case *ast.UsingStmt:
			if hit, sp, nm := usesInoutInExpr(st.Bind); hit {
				return true, sp, nm
			}
			if hit, sp, nm := usesInoutInExpr(st.Init); hit {
				return true, sp, nm
			}
			if st.Body != nil {
				for _, ss := range st.Body.Stmts {
					if hit, sp, nm := usesInoutInStmt(ss); hit {
						return true, sp, nm
					}
				}
			}
			return false, diag.Span{}, ""
		default:
			return false, diag.Span{}, ""
		}
	}

	// ---- walk body ----------------------------------------------------------

	var diags []diag.Diagnostic
	seenAwait := false
	var firstAwaitSpan diag.Span

	for _, s := range fd.Body.Stmts {
		// If we already crossed an await, any inout use now is a violation.
		if seenAwait {
			if hit, sp, nm := usesInoutInStmt(s); hit {
				diags = append(diags, makeBarrierDiag(sp, firstAwaitSpan, nm))
				break
			}
			continue
		}
		// Otherwise keep looking for the first await.
		if sp, ok := awaitSpanInStmt(s); ok {
			seenAwait = true
			firstAwaitSpan = sp
		}
	}

	return diags
}

func makeBarrierDiag(primaryWhere diag.Span, awaitWhere diag.Span, name string) diag.Diagnostic {
	d := diag.Diagnostic{
		CodeID: "DBR0001", // catalog code for "borrow.async_inout"
		Domain: "borrow",
		Primary: diag.Label{
			Span:    primaryWhere,
			Text:    "cannot hold 'inout' borrow across 'await'",
			Primary: true,
		},
	}
	if awaitWhere.File != "" {
		d.Labels = append(d.Labels, diag.Label{
			Span: awaitWhere,
			Text: "suspension point ('await')",
		})
	}
	if name != "" {
		d.Notes = append(d.Notes, "borrowed parameter: "+name)
	}
	d.FillFromCatalog()
	return d
}

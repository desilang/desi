package lower

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
)

// CheckAwaitBorrowBarrier enforces the M6/M8 rule: no unique ('inout') borrows
// may remain live across an 'await' suspension point.
//
// Tier-0 conservative implementation:
//   - If function has any ParamInout params,
//   - and there exists any Ident use of one of those params *after* the first 'await',
//   - then emit DBR0001 ("cannot hold 'inout' borrow across 'await'").
//
// We label the first post-await use as primary and the first await site as a secondary label.
//
// This returns diagnostics but does not render them; callers (or tests) can render
// using diag.RenderTTY or via the catalog builder if desired.
func CheckAwaitBorrowBarrier(fd *ast.FuncDecl) []diag.Diagnostic {
	if fd == nil || fd.Body == nil || !fd.Async {
		return nil
	}

	// Gather names of inout params.
	inout := map[string]ast.Ident{}
	for _, p := range fd.Params {
		if p.Mode == ast.ParamInout {
			inout[p.Name.Name] = p.Name
		}
	}
	if len(inout) == 0 {
		return nil
	}

	var diags []diag.Diagnostic

	// Find the first await span and then any use of inout params after that.
	var firstAwaitSpan diag.Span
	seenAwait := false

	// simple helpers ----------------------------------------------------------
	var exprHasAwait func(e ast.Expr) bool
	exprHasAwait = func(e ast.Expr) bool {
		if e == nil {
			return false
		}
		switch x := e.(type) {
		case *ast.UnaryExpr:
			if x.Op == "await" {
				return true
			}
			return exprHasAwait(x.X)
		case *ast.CallExpr:
			for _, a := range x.Args {
				if exprHasAwait(a) {
					return true
				}
			}
			return exprHasAwait(x.Callee)
		case *ast.BinaryExpr:
			return exprHasAwait(x.Lhs) || exprHasAwait(x.Rhs)
		case *ast.IndexExpr:
			return exprHasAwait(x.X) || exprHasAwait(x.Idx)
		case *ast.SliceExpr:
			if exprHasAwait(x.X) || exprHasAwait(x.I) || exprHasAwait(x.J) || exprHasAwait(x.K) {
				return true
			}
			return false
		case *ast.FieldExpr:
			return exprHasAwait(x.X)
		case *ast.LambdaExpr:
			return exprHasAwait(x.Body)
		case *ast.ListComp:
			if exprHasAwait(x.Elem) {
				return true
			}
			for _, c := range x.Clauses {
				if exprHasAwait(c.Target) || exprHasAwait(c.Iter) || exprHasAwait(c.If) {
					return true
				}
			}
			return false
		case *ast.DictComp:
			if exprHasAwait(x.Key) || exprHasAwait(x.Value) {
				return true
			}
			for _, c := range x.Clauses {
				if exprHasAwait(c.Target) || exprHasAwait(c.Iter) || exprHasAwait(c.If) {
					return true
				}
			}
			return false
		case *ast.SetComp:
			if exprHasAwait(x.Elem) {
				return true
			}
			for _, c := range x.Clauses {
				if exprHasAwait(c.Target) || exprHasAwait(c.Iter) || exprHasAwait(c.If) {
					return true
				}
			}
			return false
		default:
			return false
		}
	}

	var visitExprForPostAwaitUse func(e ast.Expr) (hit bool, where diag.Span, name string)
	visitExprForPostAwaitUse = func(e ast.Expr) (bool, diag.Span, string) {
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
			return visitExprForPostAwaitUse(x.X)
		case *ast.CallExpr:
			// check callee and args
			if hit, sp, nm := visitExprForPostAwaitUse(x.Callee); hit {
				return true, sp, nm
			}
			for _, a := range x.Args {
				if hit, sp, nm := visitExprForPostAwaitUse(a); hit {
					return true, sp, nm
				}
			}
			return false, diag.Span{}, ""
		case *ast.BinaryExpr:
			if hit, sp, nm := visitExprForPostAwaitUse(x.Lhs); hit {
				return true, sp, nm
			}
			return visitExprForPostAwaitUse(x.Rhs)
		case *ast.IndexExpr:
			if hit, sp, nm := visitExprForPostAwaitUse(x.X); hit {
				return true, sp, nm
			}
			return visitExprForPostAwaitUse(x.Idx)
		case *ast.SliceExpr:
			if hit, sp, nm := visitExprForPostAwaitUse(x.X); hit {
				return true, sp, nm
			}
			for _, part := range []ast.Expr{x.I, x.J, x.K} {
				if hit, sp, nm := visitExprForPostAwaitUse(part); hit {
					return true, sp, nm
				}
			}
			return false, diag.Span{}, ""
		case *ast.FieldExpr:
			return visitExprForPostAwaitUse(x.X)
		case *ast.LambdaExpr:
			return visitExprForPostAwaitUse(x.Body)
		case *ast.ListComp:
			if hit, sp, nm := visitExprForPostAwaitUse(x.Elem); hit {
				return true, sp, nm
			}
			for _, c := range x.Clauses {
				if hit, sp, nm := visitExprForPostAwaitUse(c.Target); hit {
					return true, sp, nm
				}
				if hit, sp, nm := visitExprForPostAwaitUse(c.Iter); hit {
					return true, sp, nm
				}
				if hit, sp, nm := visitExprForPostAwaitUse(c.If); hit {
					return true, sp, nm
				}
			}
			return false, diag.Span{}, ""
		case *ast.DictComp:
			if hit, sp, nm := visitExprForPostAwaitUse(x.Key); hit {
				return true, sp, nm
			}
			if hit, sp, nm := visitExprForPostAwaitUse(x.Value); hit {
				return true, sp, nm
			}
			for _, c := range x.Clauses {
				if hit, sp, nm := visitExprForPostAwaitUse(c.Target); hit {
					return true, sp, nm
				}
				if hit, sp, nm := visitExprForPostAwaitUse(c.Iter); hit {
					return true, sp, nm
				}
				if hit, sp, nm := visitExprForPostAwaitUse(c.If); hit {
					return true, sp, nm
				}
			}
			return false, diag.Span{}, ""
		case *ast.SetComp:
			if hit, sp, nm := visitExprForPostAwaitUse(x.Elem); hit {
				return true, sp, nm
			}
			for _, c := range x.Clauses {
				if hit, sp, nm := visitExprForPostAwaitUse(c.Target); hit {
					return true, sp, nm
				}
				if hit, sp, nm := visitExprForPostAwaitUse(c.Iter); hit {
					return true, sp, nm
				}
				if hit, sp, nm := visitExprForPostAwaitUse(c.If); hit {
					return true, sp, nm
				}
			}
			return false, diag.Span{}, ""
		default:
			return false, diag.Span{}, ""
		}
	}

	visitStmt := func(s ast.Stmt) {
		switch st := s.(type) {
		case *ast.AssignStmt:
			// RHS can contain awaits; then LHS/RHS uses after seenAwait
			for _, e := range st.RHS {
				if !seenAwait && exprHasAwait(e) {
					seenAwait = true
					firstAwaitSpan = e.SpanOf()
				}
			}
			if seenAwait {
				for _, e := range st.LHS {
					if hit, sp, nm := visitExprForPostAwaitUse(e); hit {
						diags = append(diags, makeBarrierDiag(sp, firstAwaitSpan, nm))
						return
					}
				}
				for _, e := range st.RHS {
					if hit, sp, nm := visitExprForPostAwaitUse(e); hit {
						diags = append(diags, makeBarrierDiag(sp, firstAwaitSpan, nm))
						return
					}
				}
			}
		case *ast.AugAssignStmt:
			if !seenAwait && (exprHasAwait(st.Left) || exprHasAwait(st.Right)) {
				seenAwait = true
				// prefer marking the side that contained await if known
				if exprHasAwait(st.Right) {
					firstAwaitSpan = st.Right.SpanOf()
				} else {
					firstAwaitSpan = st.Left.SpanOf()
				}
			}
			if seenAwait {
				if hit, sp, nm := visitExprForPostAwaitUse(st.Left); hit {
					diags = append(diags, makeBarrierDiag(sp, firstAwaitSpan, nm))
					return
				}
				if hit, sp, nm := visitExprForPostAwaitUse(st.Right); hit {
					diags = append(diags, makeBarrierDiag(sp, firstAwaitSpan, nm))
					return
				}
			}
		case *ast.ExprStmt:
			if !seenAwait && exprHasAwait(st.Expr) {
				seenAwait = true
				firstAwaitSpan = st.Expr.SpanOf()
			}
			if seenAwait {
				if hit, sp, nm := visitExprForPostAwaitUse(st.Expr); hit {
					diags = append(diags, makeBarrierDiag(sp, firstAwaitSpan, nm))
					return
				}
			}
		case *ast.IfStmt:
			// Condition first
			if !seenAwait && exprHasAwait(st.Cond) {
				seenAwait = true
				firstAwaitSpan = st.Cond.SpanOf()
			}
			if seenAwait {
				if hit, sp, nm := visitExprForPostAwaitUse(st.Cond); hit {
					diags = append(diags, makeBarrierDiag(sp, firstAwaitSpan, nm))
					return
				}
			}
			if st.Then != nil {
				for _, ss := range st.Then.Stmts {
					visitStmt(ss)
					if len(diags) > 0 {
						return
					}
				}
			}
			for _, a := range st.Elifs {
				if !seenAwait && exprHasAwait(a.Cond) {
					seenAwait = true
					firstAwaitSpan = a.Cond.SpanOf()
				}
				if seenAwait {
					if hit, sp, nm := visitExprForPostAwaitUse(a.Cond); hit {
						diags = append(diags, makeBarrierDiag(sp, firstAwaitSpan, nm))
						return
					}
				}
				if a.Body != nil {
					for _, ss := range a.Body.Stmts {
						visitStmt(ss)
						if len(diags) > 0 {
							return
						}
					}
				}
			}
			if st.Else != nil {
				for _, ss := range st.Else.Stmts {
					visitStmt(ss)
					if len(diags) > 0 {
						return
					}
				}
			}
		case *ast.WhileStmt:
			if !seenAwait && exprHasAwait(st.Cond) {
				seenAwait = true
				firstAwaitSpan = st.Cond.SpanOf()
			}
			if seenAwait {
				if hit, sp, nm := visitExprForPostAwaitUse(st.Cond); hit {
					diags = append(diags, makeBarrierDiag(sp, firstAwaitSpan, nm))
					return
				}
			}
			if st.Body != nil {
				for _, ss := range st.Body.Stmts {
					visitStmt(ss)
					if len(diags) > 0 {
						return
					}
				}
			}
		case *ast.ForStmt:
			if !seenAwait && (exprHasAwait(st.Target) || exprHasAwait(st.Iter)) {
				seenAwait = true
				if exprHasAwait(st.Iter) {
					firstAwaitSpan = st.Iter.SpanOf()
				} else {
					firstAwaitSpan = st.Target.SpanOf()
				}
			}
			if seenAwait {
				if hit, sp, nm := visitExprForPostAwaitUse(st.Target); hit {
					diags = append(diags, makeBarrierDiag(sp, firstAwaitSpan, nm))
					return
				}
				if hit, sp, nm := visitExprForPostAwaitUse(st.Iter); hit {
					diags = append(diags, makeBarrierDiag(sp, firstAwaitSpan, nm))
					return
				}
			}
			if st.Body != nil {
				for _, ss := range st.Body.Stmts {
					visitStmt(ss)
					if len(diags) > 0 {
						return
					}
				}
			}
		case *ast.UsingStmt:
			// 'using' init may contain awaits
			if !seenAwait && (exprHasAwait(st.Bind) || exprHasAwait(st.Init)) {
				seenAwait = true
				if exprHasAwait(st.Init) {
					firstAwaitSpan = st.Init.SpanOf()
				} else {
					firstAwaitSpan = st.Bind.SpanOf()
				}
			}
			if seenAwait {
				if hit, sp, nm := visitExprForPostAwaitUse(st.Bind); hit {
					diags = append(diags, makeBarrierDiag(sp, firstAwaitSpan, nm))
					return
				}
				if hit, sp, nm := visitExprForPostAwaitUse(st.Init); hit {
					diags = append(diags, makeBarrierDiag(sp, firstAwaitSpan, nm))
					return
				}
			}
			if st.Body != nil {
				for _, ss := range st.Body.Stmts {
					visitStmt(ss)
					if len(diags) > 0 {
						return
					}
				}
			}
		case *ast.DeferStmt:
			// calls only; not relevant for await barrier in Tier-0
		case *ast.ReturnStmt, *ast.BreakStmt, *ast.ContinueStmt, *ast.PassStmt:
			// no-op
		default:
			// keep conservative default: do nothing
		}
	}
	// Walk body in order.
	for _, s := range fd.Body.Stmts {
		visitStmt(s)
		if len(diags) > 0 {
			break
		}
	}

	return diags
}

func makeBarrierDiag(primaryWhere diag.Span, awaitWhere diag.Span, name string) diag.Diagnostic {
	d := diag.Diagnostic{
		CodeID: "DBR0001", // from codes.json ("borrow.async_inout")
		Domain: "borrow",
		Primary: diag.Label{
			Span:    primaryWhere,
			Text:    "cannot hold 'inout' borrow across 'await'",
			Primary: true,
		},
	}
	// Add secondary label to show the suspension point for context.
	if awaitWhere.File != "" {
		d.Labels = append(d.Labels, diag.Label{
			Span: awaitWhere,
			Text: "suspension point ('await')",
		})
	}
	if name != "" {
		d.Notes = append(d.Notes, "borrowed parameter: "+name)
	}
	// Ask catalog to fill Title/Help where available when rendering.
	d.FillFromCatalog()
	return d
}

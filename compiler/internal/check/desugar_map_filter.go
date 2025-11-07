package check

import "github.com/desilang/desi/compiler/internal/ast"

// desugarMapFilter rewrites map(xs,f) and filter(xs,p) into list comprehensions:
//
//	map(xs,f)    → [f(x) for x in xs]
//	filter(xs,p) → [x for x in xs if p(x)]
//
// It mutates the given module in-place and runs before type checking.
func desugarMapFilter(mod *ast.Module) {
	if mod == nil {
		return
	}
	for _, d := range mod.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		desugarBlock(fd.Body)
	}
}

func desugarBlock(b *ast.Block) {
	if b == nil {
		return
	}
	for i := range b.Stmts {
		switch s := b.Stmts[i].(type) {
		case *ast.ExprStmt:
			s.Expr = desugarExpr(s.Expr)
		case *ast.LetStmt:
			if s.Value != nil {
				s.Value = desugarExpr(s.Value)
			}
		case *ast.ReturnStmt:
			if s.Value != nil {
				s.Value = desugarExpr(s.Value)
			}
		case *ast.AssignStmt:
			for j := range s.LHS {
				s.LHS[j] = desugarExpr(s.LHS[j])
			}
			for j := range s.RHS {
				s.RHS[j] = desugarExpr(s.RHS[j])
			}
		case *ast.IfStmt:
			s.Cond = desugarExpr(s.Cond)
			desugarBlock(s.Then)
			for j := range s.Elifs {
				s.Elifs[j].Cond = desugarExpr(s.Elifs[j].Cond)
				desugarBlock(s.Elifs[j].Body)
			}
			desugarBlock(s.Else)
		case *ast.WhileStmt:
			s.Cond = desugarExpr(s.Cond)
			desugarBlock(s.Body)
		case *ast.UsingStmt:
			if s.Init != nil {
				s.Init = desugarExpr(s.Init)
			}
			desugarBlock(s.Body)
		case *ast.DeferStmt:
			if s.Call != nil {
				s.Call = desugarExpr(s.Call).(*ast.CallExpr)
			}
		}
	}
}

func desugarExpr(e ast.Expr) ast.Expr {
	switch x := e.(type) {
	case *ast.CallExpr:
		// First, desugar inside callee/args.
		callee := desugarExpr(x.Callee)
		args := make([]ast.Expr, len(x.Args))
		for i, a := range x.Args {
			args[i] = desugarExpr(a)
		}
		x.Callee, x.Args = callee, args

		// Then, check for map/filter shapes (2-arg only).
		if id, ok := x.Callee.(*ast.Ident); ok && len(x.Args) == 2 {
			switch id.Name {
			case "map":
				lc := buildMapComp(args[0], args[1]).(*ast.ListComp)
				// Preserve the outer call's span so diagnostics point to the call.
				lc.Span = x.SpanOf()
				return lc
			case "filter":
				lc := buildFilterComp(args[0], args[1]).(*ast.ListComp)
				lc.Span = x.SpanOf()
				return lc
			}
		}
		return x

	case *ast.ListComp:
		// Recurse into comprehension parts
		x.Elem = desugarExpr(x.Elem)
		for i := range x.Clauses {
			cl := &x.Clauses[i]
			cl.Iter = desugarExpr(cl.Iter)
			if cl.If != nil {
				cl.If = desugarExpr(cl.If)
			}
		}
		return x

	case *ast.DictComp:
		x.Key = desugarExpr(x.Key)
		x.Val = desugarExpr(x.Val)
		for i := range x.Clauses {
			cl := &x.Clauses[i]
			cl.Iter = desugarExpr(cl.Iter)
			if cl.If != nil {
				cl.If = desugarExpr(cl.If)
			}
		}
		return x

	case *ast.FieldExpr:
		x.X = desugarExpr(x.X)
		return x

	default:
		return x
	}
}

func buildMapComp(xs ast.Expr, f ast.Expr) ast.Expr {
	// [ f(__x) for __x in xs ]
	xvar := &ast.Ident{Name: "__x"}
	call := &ast.CallExpr{Callee: f, Args: []ast.Expr{xvar}}
	return &ast.ListComp{
		Elem: call,
		Clauses: []ast.CompClause{{
			Target: xvar,
			Iter:   xs,
			If:     nil,
		}},
	}
}

func buildFilterComp(xs ast.Expr, p ast.Expr) ast.Expr {
	// [ __x for __x in xs if p(__x) ]
	xvar := &ast.Ident{Name: "__x"}
	pred := &ast.CallExpr{Callee: p, Args: []ast.Expr{xvar}}
	return &ast.ListComp{
		Elem: xvar,
		Clauses: []ast.CompClause{{
			Target: xvar,
			Iter:   xs,
			If:     pred,
		}},
	}
}

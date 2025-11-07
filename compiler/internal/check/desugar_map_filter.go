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

		// Then, check for map/filter shapes.
		if id, ok := x.Callee.(*ast.Ident); ok && len(x.Args) == 2 {
			switch id.Name {
			case "map":
				return buildMapComp(args[0], args[1])
			case "filter":
				return buildFilterComp(args[0], args[1])
			}
		}
		return x

	case *ast.UnaryExpr:
		x.X = desugarExpr(x.X)
		return x
	case *ast.BinaryExpr:
		x.Lhs = desugarExpr(x.Lhs)
		x.Rhs = desugarExpr(x.Rhs)
		return x
	case *ast.IndexExpr:
		x.X = desugarExpr(x.X)
		x.Idx = desugarExpr(x.Idx)
		return x
	case *ast.FieldExpr:
		x.X = desugarExpr(x.X)
		return x
	case *ast.ListComp:
		// Recurse into list comp pieces.
		x.Elem = desugarExpr(x.Elem)
		for i := range x.Clauses {
			x.Clauses[i].Target = desugarExpr(x.Clauses[i].Target)
			x.Clauses[i].Iter = desugarExpr(x.Clauses[i].Iter)
			if x.Clauses[i].If != nil {
				x.Clauses[i].If = desugarExpr(x.Clauses[i].If)
			}
		}
		return x
	case *ast.DictComp:
		x.Key = desugarExpr(x.Key)
		x.Val = desugarExpr(x.Val)
		for i := range x.Clauses {
			x.Clauses[i].Target = desugarExpr(x.Clauses[i].Target)
			x.Clauses[i].Iter = desugarExpr(x.Clauses[i].Iter)
			if x.Clauses[i].If != nil {
				x.Clauses[i].If = desugarExpr(x.Clauses[i].If)
			}
		}
		return x
	case *ast.SetComp:
		x.Elem = desugarExpr(x.Elem)
		for i := range x.Clauses {
			x.Clauses[i].Target = desugarExpr(x.Clauses[i].Target)
			x.Clauses[i].Iter = desugarExpr(x.Clauses[i].Iter)
			if x.Clauses[i].If != nil {
				x.Clauses[i].If = desugarExpr(x.Clauses[i].If)
			}
		}
		return x
	default:
		return x
	}
}

func buildMapComp(xs ast.Expr, f ast.Expr) ast.Expr {
	it := &ast.Ident{Name: "__x"}
	call := &ast.CallExpr{Callee: f, Args: []ast.Expr{&ast.Ident{Name: it.Name}}}
	return &ast.ListComp{
		Elem: call,
		Clauses: []ast.CompClause{{
			Target: it,
			Iter:   xs,
			If:     nil,
		}},
	}
}

func buildFilterComp(xs ast.Expr, p ast.Expr) ast.Expr {
	it := &ast.Ident{Name: "__x"}
	cond := &ast.CallExpr{Callee: p, Args: []ast.Expr{&ast.Ident{Name: it.Name}}}
	return &ast.ListComp{
		Elem: it,
		Clauses: []ast.CompClause{{
			Target: it,
			Iter:   xs,
			If:     cond,
		}},
	}
}

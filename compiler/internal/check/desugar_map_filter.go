package check

import "github.com/desilang/desi/compiler/internal/ast"

// =============================================================================
// DESUGARING ARCHITECTURE - DO NOT REMOVE THIS COMMENT
// =============================================================================
//
// Desugaring transforms high-level syntax into simpler forms before type-checking.
// This happens at compile-time with ZERO runtime overhead - the generated code is
// identical to hand-written equivalents.
//
// PERFORMANCE: Currently O(n) where n = number of AST nodes.
//
// ADDING NEW DESUGARS:
// To maintain O(n) complexity when adding new desugar transformations:
//
// 1. Add your pattern to desugarExpr() switch statement (preferred)
//    - This keeps everything in a single AST walk
//    - Example: add a case for 'reduce', 'flatmap', etc.
//
// 2. If the transformation is complex, add a new case to desugarBlock()
//    - Still O(n) as it's part of the same walk
//
// 3. AVOID creating separate AST walks for each desugar type
//    - Multiple walks = O(k*n) where k = number of desugar types
//    - Only acceptable if desugars have ordering dependencies
//
// See docs/internals/desugaring.md for design rationale and examples.
// =============================================================================

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
	case *ast.BinaryExpr:
		// Recurse into both sides first
		x.Lhs = desugarExpr(x.Lhs)
		x.Rhs = desugarExpr(x.Rhs)

		// Handle pipe operator: xs |> map(f), xs |> filter(p)
		// BUT skip desugaring for Option/Result types (they have their own map method)
		if x.Op == "|>" {
			if call, ok := x.Rhs.(*ast.CallExpr); ok {
				if id, ok := call.Callee.(*ast.Ident); ok && len(call.Args) == 1 {
					// Skip if LHS looks like Option or Result
					if !looksLikeOptionOrResult(x.Lhs) {
						switch id.Name {
						case "map":
							// xs |> map(f) → [f(__x) for __x in xs]
							lc := buildMapComp(x.Lhs, call.Args[0]).(*ast.ListComp)
							lc.Span = x.SpanOf()
							return lc
						case "filter":
							// xs |> filter(p) → [__x for __x in xs if p(__x)]
							lc := buildFilterComp(x.Lhs, call.Args[0]).(*ast.ListComp)
							lc.Span = x.SpanOf()
							return lc
						}
					}
				}
			}
		}
		return x

	case *ast.CallExpr:
		// First, desugar inside callee/args.
		callee := desugarExpr(x.Callee)
		args := make([]ast.Expr, len(x.Args))
		for i, a := range x.Args {
			args[i] = desugarExpr(a)
		}
		x.Callee, x.Args = callee, args

		// Handle dot method syntax: xs.map(f), xs.filter(p)
		// BUT skip desugaring for Option/Result types (they have their own map method)
		if fe, ok := x.Callee.(*ast.FieldExpr); ok && len(x.Args) == 1 {
			// Skip if receiver looks like Option or Result (AST pattern check since types aren't available yet)
			if !looksLikeOptionOrResult(fe.X) {
				switch fe.Name.Name {
				case "map":
					// xs.map(f) → [f(__x) for __x in xs]
					lc := buildMapComp(fe.X, x.Args[0]).(*ast.ListComp)
					lc.Span = x.SpanOf()
					return lc
				case "filter":
					// xs.filter(p) → [__x for __x in xs if p(__x)]
					lc := buildFilterComp(fe.X, x.Args[0]).(*ast.ListComp)
					lc.Span = x.SpanOf()
					return lc
				}
			}
		}

		// Then, check for map/filter shapes (2-arg only).
		// Python 3 order: map(func, iterable), filter(func, iterable)
		if id, ok := x.Callee.(*ast.Ident); ok && len(x.Args) == 2 {
			switch id.Name {
			case "map":
				// args[0] = func, args[1] = iterable (Python order)
				lc := buildMapComp(args[1], args[0]).(*ast.ListComp)
				// Preserve the outer call's span so diagnostics point to the call.
				lc.Span = x.SpanOf()
				return lc
			case "filter":
				// args[0] = predicate, args[1] = iterable (Python order)
				lc := buildFilterComp(args[1], args[0]).(*ast.ListComp)
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

// looksLikeOptionOrResult checks if an expression appears to be an Option or Result
// value based on AST patterns. Since desugaring happens before type checking,
// we can't know the actual type, so we use heuristics:
//
//  1. Direct constructor: Option.Some(...), Option.Nothing, Result.Ok(...), Result.Err(...)
//  2. Method call on something that looks like Option/Result: x.unwrap(), x.is_some(), etc.
//  3. Variables with common naming patterns (defensive, catches common cases)
//
// This is necessary to prevent list comprehension desugaring from capturing Option.map/Result.map.
// Zero runtime overhead - this is compile-time only.
func looksLikeOptionOrResult(e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.Ident:
		// Check if identifier name suggests Option/Result (common patterns)
		// This is heuristic-based for simple variable names
		name := x.Name
		// Skip if it's a likely collection name
		if name == "xs" || name == "items" || name == "list" || name == "arr" || name == "data" {
			return false
		}
		// Check for Option/Result related names
		if name == "opt" || name == "option" || name == "maybe" ||
			name == "res" || name == "result" ||
			name == "some" || name == "none" || name == "nothing" ||
			name == "ok" || name == "err" {
			return true
		}
		return false

	case *ast.FieldExpr:
		// Check for Option.Something or Result.Something
		if id, ok := x.X.(*ast.Ident); ok {
			if id.Name == "Option" || id.Name == "Result" {
				return true
			}
		}
		// Check for method calls that are Option/Result specific: x.unwrap(), x.is_some(), etc.
		methodName := x.Name.Name
		if methodName == "unwrap" || methodName == "unwrap_or" || methodName == "expect" ||
			methodName == "is_some" || methodName == "is_nothing" || methodName == "is_none" ||
			methodName == "is_ok" || methodName == "is_err" || methodName == "ok" || methodName == "err" {
			return true
		}
		return looksLikeOptionOrResult(x.X)

	case *ast.CallExpr:
		// Check if this is an Option/Result constructor call
		if fe, ok := x.Callee.(*ast.FieldExpr); ok {
			if id, ok := fe.X.(*ast.Ident); ok {
				if id.Name == "Option" || id.Name == "Result" {
					return true
				}
			}
		}
		// Check callee for Option/Result method chains
		return looksLikeOptionOrResult(x.Callee)

	default:
		return false
	}
}

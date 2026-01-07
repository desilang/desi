package lower

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
)

// DesugarAsyncLambdas finds lambda expressions inside function bodies,
// synthesizes hidden functions named "__lam$N", and replaces the lambda
// expression with an Ident("__lam$N") at the callsite.
//
// The synthesized function body is a single `return <lambda-body>`; Params are
// copied (converted) from the lambda. Names are made unique within the module.
//
// Returns a map of variable names to hidden function names for lambdas assigned to variables.
func DesugarAsyncLambdas(mod *ast.Module) map[string]string {
	aliases := make(map[string]string)
	if mod == nil {
		return aliases
	}

	// Start counter after the largest existing __lam$N to keep names stable.
	next := 0
	for _, d := range mod.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok {
			if strings.HasPrefix(fd.Name.Name, "__lam$") {
				if n, err := strconv.Atoi(strings.TrimPrefix(fd.Name.Name, "__lam$")); err == nil && n >= next {
					next = n + 1
				}
			}
		}
	}

	var synth []*ast.FuncDecl

	// Walk every function's body and rewrite in place.
	for _, d := range mod.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		for i, s := range fd.Body.Stmts {
			fd.Body.Stmts[i] = rewriteStmtForAsyncLambda(s, mod, &synth, &next, aliases)
		}
	}

	// Append all synthesized functions to the module.
	if len(synth) > 0 {
		for _, f := range synth {
			mod.Decls = append(mod.Decls, f)
		}
	}
	return aliases
}

func rewriteStmtForAsyncLambda(s ast.Stmt, mod *ast.Module, synth *[]*ast.FuncDecl, next *int, aliases map[string]string) ast.Stmt {
	switch st := s.(type) {
	case *ast.LetStmt:
		// Handle let x = lambda<...>...
		if st.Value != nil {
			st.Value = rewriteExprForAsyncLambda(st.Value, mod, synth, next)
			// If value is now a __lam$N identifier, record the alias
			if id, ok := st.Value.(*ast.Ident); ok && strings.HasPrefix(id.Name, "__lam$") {
				aliases[st.Name.Name] = id.Name
			}
		}
		return st
	case *ast.AssignStmt:
		for i, e := range st.LHS {
			st.LHS[i] = rewriteExprForAsyncLambda(e, mod, synth, next)
		}
		for i, e := range st.RHS {
			st.RHS[i] = rewriteExprForAsyncLambda(e, mod, synth, next)
		}
		return st
	case *ast.ExprStmt:
		st.Expr = rewriteExprForAsyncLambda(st.Expr, mod, synth, next)
		return st
	case *ast.ReturnStmt:
		st.Value = rewriteExprForAsyncLambda(st.Value, mod, synth, next)
		return st
	case *ast.IfStmt:
		st.Cond = rewriteExprForAsyncLambda(st.Cond, mod, synth, next)
		if st.Then != nil {
			for i, ss := range st.Then.Stmts {
				st.Then.Stmts[i] = rewriteStmtForAsyncLambda(ss, mod, synth, next, aliases)
			}
		}
		for _, e := range st.Elifs {
			e.Cond = rewriteExprForAsyncLambda(e.Cond, mod, synth, next)
			if e.Body != nil {
				for i, ss := range e.Body.Stmts {
					e.Body.Stmts[i] = rewriteStmtForAsyncLambda(ss, mod, synth, next, aliases)
				}
			}
		}
		if st.Else != nil {
			for i, ss := range st.Else.Stmts {
				st.Else.Stmts[i] = rewriteStmtForAsyncLambda(ss, mod, synth, next, aliases)
			}
		}
		return st
	case *ast.WhileStmt:
		st.Cond = rewriteExprForAsyncLambda(st.Cond, mod, synth, next)
		if st.Body != nil {
			for i, ss := range st.Body.Stmts {
				st.Body.Stmts[i] = rewriteStmtForAsyncLambda(ss, mod, synth, next, aliases)
			}
		}
		return st
	case *ast.UsingStmt:
		st.Bind = rewriteExprForAsyncLambda(st.Bind, mod, synth, next)
		st.Init = rewriteExprForAsyncLambda(st.Init, mod, synth, next)
		if st.Body != nil {
			for i, ss := range st.Body.Stmts {
				st.Body.Stmts[i] = rewriteStmtForAsyncLambda(ss, mod, synth, next, aliases)
			}
		}
		return st
	default:
		return st
	}
}

func rewriteExprForAsyncLambda(e ast.Expr, mod *ast.Module, synth *[]*ast.FuncDecl, next *int) ast.Expr {
	switch x := e.(type) {
	case *ast.LambdaExpr:
		// Recurse into body first (in case of nested lambdas).
		x.Body = rewriteExprForAsyncLambda(x.Body, mod, synth, next)

		// Build scope from lambda parameters
		scope := make(map[string]bool)
		for _, lp := range x.Params {
			scope[lp.Name.Name] = true
		}

		// Collect captured (free) variables from lambda body
		captures := CollectFreeVars(x.Body, scope)

		// Synthesize: def __lam$N(params..., captures...) -> RetType: return <body>
		lamName := fmt.Sprintf("__lam$%d", *next)
		*next++

		// Convert lambda params -> function params (including captures as extra params)
		params := make([]ast.Param, 0, len(x.Params)+len(captures))
		for _, lp := range x.Params {
			params = append(params, ast.Param{Name: lp.Name, Type: lp.Type})
		}
		for _, cap := range captures {
			params = append(params, ast.Param{
				Name: ast.Ident{Name: cap},
				Type: nil, // Type inferred at callsite
			})
		}

		// Build function body based on return type
		var bodyStmts []ast.Stmt
		if x.RetType != nil && x.RetType.Name == "none" {
			bodyStmts = []ast.Stmt{
				&ast.ExprStmt{Expr: x.Body},
				&ast.ReturnStmt{Value: nil},
			}
		} else {
			bodyStmts = []ast.Stmt{
				&ast.ReturnStmt{Value: x.Body},
			}
		}

		fn := &ast.FuncDecl{
			Async:   x.Async,
			Name:    ast.Ident{Name: lamName},
			Params:  params,
			RetType: x.RetType,
			Body:    &ast.Block{Stmts: bodyStmts},
		}
		*synth = append(*synth, fn)

		// If there are captures, return capture marker with lambda name
		// The tg.run() lowering will generate wrapper and handle ctx passing
		if len(captures) > 0 {
			args := make([]ast.Expr, len(captures))
			for i, cap := range captures {
				args[i] = &ast.Ident{Name: cap}
			}
			// Return: lamName.__captures__(a, b, c)
			return &ast.CallExpr{
				Callee: &ast.FieldExpr{
					X:    &ast.Ident{Name: lamName},
					Name: ast.Ident{Name: "__captures__"},
				},
				Args: args,
			}
		}

		// No captures - just return lambda identifier for regular lambda calls
		return &ast.Ident{Name: lamName}

	case *ast.CallExpr:
		x.Callee = rewriteExprForAsyncLambda(x.Callee, mod, synth, next)
		for i, a := range x.Args {
			x.Args[i] = rewriteExprForAsyncLambda(a, mod, synth, next)
		}
		return x
	case *ast.UnaryExpr:
		x.X = rewriteExprForAsyncLambda(x.X, mod, synth, next)
		return x
	case *ast.BinaryExpr:
		x.Lhs = rewriteExprForAsyncLambda(x.Lhs, mod, synth, next)
		x.Rhs = rewriteExprForAsyncLambda(x.Rhs, mod, synth, next)
		return x
	case *ast.IndexExpr:
		x.X = rewriteExprForAsyncLambda(x.X, mod, synth, next)
		x.Idx = rewriteExprForAsyncLambda(x.Idx, mod, synth, next)
		return x
	case *ast.FieldExpr:
		x.X = rewriteExprForAsyncLambda(x.X, mod, synth, next)
		return x
	default:
		return e
	}
}

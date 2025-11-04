package lower

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
)

// DesugarAsyncLambdas finds `async lambda` expressions inside function bodies,
// synthesizes hidden async functions named "__lam$N", and replaces the lambda
// expression with an Ident("__lam$N") at the callsite.
//
// The synthesized function body is a single `return <lambda-body>`; Params are
// copied (converted) from the lambda. Names are made unique within the module.
func DesugarAsyncLambdas(mod *ast.Module) {
	if mod == nil {
		return
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
			fd.Body.Stmts[i] = rewriteStmtForAsyncLambda(s, mod, &synth, &next)
		}
	}

	// Append all synthesized functions to the module.
	if len(synth) > 0 {
		for _, f := range synth {
			mod.Decls = append(mod.Decls, f)
		}
	}
}

func rewriteStmtForAsyncLambda(s ast.Stmt, mod *ast.Module, synth *[]*ast.FuncDecl, next *int) ast.Stmt {
	switch st := s.(type) {
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
				st.Then.Stmts[i] = rewriteStmtForAsyncLambda(ss, mod, synth, next)
			}
		}
		for _, e := range st.Elifs {
			e.Cond = rewriteExprForAsyncLambda(e.Cond, mod, synth, next)
			if e.Body != nil {
				for i, ss := range e.Body.Stmts {
					e.Body.Stmts[i] = rewriteStmtForAsyncLambda(ss, mod, synth, next)
				}
			}
		}
		if st.Else != nil {
			for i, ss := range st.Else.Stmts {
				st.Else.Stmts[i] = rewriteStmtForAsyncLambda(ss, mod, synth, next)
			}
		}
		return st
	case *ast.WhileStmt:
		st.Cond = rewriteExprForAsyncLambda(st.Cond, mod, synth, next)
		if st.Body != nil {
			for i, ss := range st.Body.Stmts {
				st.Body.Stmts[i] = rewriteStmtForAsyncLambda(ss, mod, synth, next)
			}
		}
		return st
	case *ast.UsingStmt:
		st.Bind = rewriteExprForAsyncLambda(st.Bind, mod, synth, next)
		st.Init = rewriteExprForAsyncLambda(st.Init, mod, synth, next)
		if st.Body != nil {
			for i, ss := range st.Body.Stmts {
				st.Body.Stmts[i] = rewriteStmtForAsyncLambda(ss, mod, synth, next)
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

		if !x.Async {
			return x
		}
		// Convert lambda params -> function params
		params := make([]ast.Param, len(x.Params))
		for i, lp := range x.Params {
			params[i] = ast.Param{Name: lp.Name, Type: lp.Type}
		}

		// Synthesize: async def __lam$N(params): return <body>
		name := fmt.Sprintf("__lam$%d", *next)
		*next++

		fn := &ast.FuncDecl{
			Async:  true,
			Name:   ast.Ident{Name: name},
			Params: params,
			Body: &ast.Block{
				Stmts: []ast.Stmt{
					&ast.ReturnStmt{Value: x.Body},
				},
			},
		}
		*synth = append(*synth, fn)

		// Replace the lambda node at the callsite with an identifier.
		return &ast.Ident{Name: name}

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

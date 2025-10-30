package check

import (
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
)

// detectAwait reports whether a subtree contains any 'await' expression.
func detectAwait(n ast.Node) bool {
	if n == nil {
		return false
	}
	return len(collectAwaitSpans(n)) > 0
}

// collectAwaitSpans walks the subtree and returns spans for all UnaryExpr(Op=="await").
func collectAwaitSpans(n ast.Node) []diag.Span {
	var out []diag.Span

	var walkExpr func(e ast.Expr)
	var walkStmt func(s ast.Stmt)
	var walkBlock func(b *ast.Block)

	walkExpr = func(e ast.Expr) {
		if e == nil {
			return
		}
		switch x := e.(type) {
		case *ast.UnaryExpr:
			if x.Op == "await" {
				out = append(out, x.Span)
			}
			walkExpr(x.X)
		case *ast.BinaryExpr:
			walkExpr(x.Lhs)
			walkExpr(x.Rhs)
		case *ast.CallExpr:
			walkExpr(x.Callee)
			for _, a := range x.Args {
				walkExpr(a)
			}
		case *ast.FieldExpr:
			walkExpr(x.X)
		case *ast.IndexExpr:
			walkExpr(x.X)
			walkExpr(x.Idx)
		case *ast.SliceExpr:
			walkExpr(x.X)
			walkExpr(x.I)
			walkExpr(x.J)
			walkExpr(x.K)
		case *ast.LambdaExpr:
			walkExpr(x.Body)
		case *ast.ListComp:
			walkExpr(x.Elem)
			for _, c := range x.Clauses {
				walkExpr(c.Target)
				walkExpr(c.Iter)
				walkExpr(c.If)
			}
		case *ast.DictComp:
			walkExpr(x.Key)
			walkExpr(x.Val)
			for _, c := range x.Clauses {
				walkExpr(c.Target)
				walkExpr(c.Iter)
				walkExpr(c.If)
			}
		case *ast.SetComp:
			walkExpr(x.Elem)
			for _, c := range x.Clauses {
				walkExpr(c.Target)
				walkExpr(c.Iter)
				walkExpr(c.If)
			}
		default:
			// literals/idents: no children
		}
	}

	walkStmt = func(s ast.Stmt) {
		if s == nil {
			return
		}
		switch st := s.(type) {
		case *ast.ExprStmt:
			walkExpr(st.Expr)
		case *ast.ReturnStmt:
			walkExpr(st.Value)
		case *ast.LetStmt:
			walkExpr(st.Value)
		case *ast.AssignStmt:
			for _, e := range st.LHS {
				walkExpr(e)
			}
			for _, e := range st.RHS {
				walkExpr(e)
			}
		case *ast.AugAssignStmt:
			walkExpr(st.Left)
			walkExpr(st.Right)
		case *ast.IfStmt:
			walkExpr(st.Cond)
			walkBlock(st.Then)
			for _, arm := range st.Elifs {
				walkExpr(arm.Cond)
				walkBlock(arm.Body)
			}
			walkBlock(st.Else)
		case *ast.WhileStmt:
			walkExpr(st.Cond)
			walkBlock(st.Body)
		case *ast.ForStmt:
			walkExpr(st.Target)
			walkExpr(st.Iter)
			walkBlock(st.Body)
		case *ast.UsingStmt:
			walkExpr(st.Bind)
			walkExpr(st.Init)
			walkBlock(st.Body)
		case *ast.DeferStmt:
			if st.Call != nil {
				walkExpr(st.Call)
			}
		case *ast.MatchStmt:
			walkExpr(st.Scrutinee)
			for _, arm := range st.Arms {
				walkExpr(arm.Pattern)
				walkExpr(arm.Result)
			}
		case *ast.DocStringStmt:
			// ignore
		default:
			// no-op
		}
	}

	walkBlock = func(b *ast.Block) {
		if b == nil {
			return
		}
		for _, s := range b.Stmts {
			walkStmt(s)
		}
	}

	// Dispatch from root
	switch nn := n.(type) {
	case *ast.Block:
		walkBlock(nn)
	case ast.Stmt:
		walkStmt(nn)
	case ast.Expr:
		walkExpr(nn)
	default:
		if fd, ok := n.(*ast.FuncDecl); ok {
			walkBlock(fd.Body)
		}
	}

	return out
}

// checkAsyncInoutAwait: in async functions, if any inout params are present
// and the body contains 'await', emit DBR0001 at each await site.
func (c *checker) checkAsyncInoutAwait(fn *ast.FuncDecl) {
	if fn == nil || !fn.Async || fn.Body == nil {
		return
	}

	// Collect inout parameters.
	var inout []string
	for i := range fn.Params {
		p := fn.Params[i]
		if p.Mode == ast.ParamInout {
			inout = append(inout, p.Name.Name)
		}
	}
	if len(inout) == 0 {
		return
	}

	awaits := collectAwaitSpans(fn.Body)
	if len(awaits) == 0 {
		return
	}

	names := strings.Join(inout, ", ")
	for _, sp := range awaits {
		c.add(diagAt("DBR0001", sp, "cannot hold 'inout' borrow across 'await' (parameter(s): "+names+")"))
	}
}

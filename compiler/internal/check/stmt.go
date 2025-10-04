package check

import (
	"fmt"

	"github.com/desilang/desi/compiler/internal/ast"
)

/* ---------- statements ---------- */

func (c *checker) checkStmt(s ast.Stmt) {
	if br := top(c.blockReturned); br != nil && *br {
		c.warnings = append(c.warnings, Warning{
			Code: CodeUnreachableCode(),
			Msg:  "unreachable code: statement after return",
		})
	}

	switch st := s.(type) {
	case *ast.LetStmt:
		c.checkLet(st)

	case *ast.AssignStmt:
		c.checkAssign(st)

	case *ast.ReturnStmt:
		exp := c.fnSig.Ret
		if st.Expr == nil {
			if exp != KindVoid {
				c.errors = append(c.errors, ErrWrongReturnKindAt(st.Span, fmt.Sprintf("%s", exp), "void", "return"))
			}
			if br := top(c.blockReturned); br != nil {
				*br = true
			}
			return
		}
		got := c.kindOfExpr(st.Expr)
		if exp == KindVoid {
			c.errors = append(c.errors, ErrWrongReturnKindAt(st.Span, "void", fmt.Sprintf("%s", got), "return"))
			if br := top(c.blockReturned); br != nil {
				*br = true
			}
			return
		}
		if _, ok := unifyKinds(exp, got); !ok {
			c.errors = append(c.errors, ErrWrongReturnKindAt(st.Span, fmt.Sprintf("%s", exp), fmt.Sprintf("%s", got), "return"))
		}
		if br := top(c.blockReturned); br != nil {
			*br = true
		}

	case *ast.ExprStmt:
		c.kindOfExpr(st.Expr)

	case *ast.IfStmt:
		k := c.kindOfExpr(st.Cond)
		if k != KindBool && k != KindInt && k != KindUnknown {
			c.errors = append(c.errors, ErrBadConditionTypeAt(st.Span, "if", k))
		}
		c.withBlock(func() {
			for _, s2 := range st.Then {
				c.checkStmt(s2)
			}
		})
		for _, el := range st.Elifs {
			k := c.kindOfExpr(el.Cond)
			if k != KindBool && k != KindInt && k != KindUnknown {
				c.errors = append(c.errors, ErrBadConditionTypeAt(el.Span, "elif", k))
			}
			c.withBlock(func() {
				for _, s2 := range el.Body {
					c.checkStmt(s2)
				}
			})
		}
		if st.Else != nil {
			c.withBlock(func() {
				for _, s2 := range st.Else {
					c.checkStmt(s2)
				}
			})
		}

	case *ast.WhileStmt:
		k := c.kindOfExpr(st.Cond)
		if k != KindBool && k != KindInt && k != KindUnknown {
			c.errors = append(c.errors, ErrBadConditionTypeAt(st.Span, "while", k))
		}
		c.withBlock(func() {
			for _, s2 := range st.Body {
				c.checkStmt(s2)
			}
		})

	case *ast.DeferStmt:
		if len(c.blockReturned) > 1 {
			c.errors = append(c.errors, ErrIllegalDeferPositionAt(st.Span))
		}
		if _, ok := st.Call.(*ast.CallExpr); !ok {
			c.errors = append(c.errors, ErrDeferExpectsCallAt(st.Span))
		}
		c.kindOfExpr(st.Call)

	case *ast.MatchStmt:
		c.checkMatch(st)
	}
}

func (c *checker) withChildScope(body func()) {
	prev := c.scope
	c.scope = &scope{parent: prev, vars: map[string]*varInfo{}}
	body()
	c.scope = prev
}

func (c *checker) withBlock(body func()) {
	c.blockReturned = push(c.blockReturned, false)
	c.withChildScope(body)
	c.blockReturned = pop(c.blockReturned)
}

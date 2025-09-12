package check

import (
	"fmt"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
)

/* ---------- statements ---------- */

func (c *checker) checkStmt(s ast.Stmt) {
	if br := top(c.blockReturned); br != nil && *br {
		c.warnings = append(c.warnings, Warning{
			Code: "W0004",
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
				c.errors = append(c.errors, fmt.Errorf("missing return value; function returns %s", exp))
			}
			if br := top(c.blockReturned); br != nil {
				*br = true
			}
			return
		}
		got := c.kindOfExpr(st.Expr)
		if exp == KindVoid {
			c.errors = append(c.errors, fmt.Errorf("return value in function returning void"))
			if br := top(c.blockReturned); br != nil {
				*br = true
			}
			return
		}
		if _, ok := unifyKinds(exp, got); !ok {
			c.errors = append(c.errors, fmt.Errorf("return kind mismatch: have %s, got %s", exp, got))
		}
		if br := top(c.blockReturned); br != nil {
			*br = true
		}
	case *ast.ExprStmt:
		c.kindOfExpr(st.Expr)
	case *ast.IfStmt:
		k := c.kindOfExpr(st.Cond)
		if k != KindBool && k != KindInt && k != KindUnknown {
			c.errors = append(c.errors, fmt.Errorf("if-condition must be bool/int, got %s", k))
		}
		c.withBlock(func() {
			for _, s2 := range st.Then {
				c.checkStmt(s2)
			}
		})
		for _, el := range st.Elifs {
			k := c.kindOfExpr(el.Cond)
			if k != KindBool && k != KindInt && k != KindUnknown {
				c.errors = append(c.errors, fmt.Errorf("elif-condition must be bool/int, got %s", k))
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
			c.errors = append(c.errors, fmt.Errorf("while-condition must be bool/int, got %s", k))
		}
		c.withBlock(func() {
			for _, s2 := range st.Body {
				c.checkStmt(s2)
			}
		})
	case *ast.DeferStmt:
		if len(c.blockReturned) > 1 {
			c.errors = append(c.errors, fmt.Errorf("defer is only allowed at function top-level in Stage-0"))
		}
		if _, ok := st.Call.(*ast.CallExpr); !ok {
			c.errors = append(c.errors, fmt.Errorf("defer expects a call expression"))
		}
		c.kindOfExpr(st.Call)
	}
}

func (c *checker) checkLet(st *ast.LetStmt) {
	// Arity check
	if len(st.Binds) != len(st.Values) {
		c.errors = append(c.errors, typedErr(
			"type", "arity_mismatch", "DTE0002", "arity mismatch in grouped binding",
			"let", len(st.Binds), len(st.Values),
		))
		// still attempt to check pairs we do have
	}

	_max := min(len(st.Binds), len(st.Values))
	for i := 0; i < _max; i++ {
		bd := st.Binds[i]
		rk := c.kindOfExpr(st.Values[i])

		var want Kind = KindUnknown
		if strings.TrimSpace(bd.Type) != "" {
			want = mapTextType(bd.Type)
		}
		kind := rk
		if want != KindUnknown {
			if k, ok := unifyKinds(want, rk); ok {
				kind = k
			} else {
				c.errors = append(c.errors, fmt.Errorf("let %q: type mismatch (declared %s, got %s)", bd.Name, want, rk))
			}
		}

		v := &varInfo{kind: kind, mutable: st.Mutable, declName: bd.Name, written: true}
		if err := c.scope.define(bd.Name, v); err != nil {
			c.errors = append(c.errors, err)
		} else {
			c.locals = append(c.locals, v)
		}
	}

	// Optional group type currently informational — future work: check tuple type vs. RHS.
	if strings.TrimSpace(st.GroupType) != "" {
		// No-op for now.
	}
}

func (c *checker) checkAssign(st *ast.AssignStmt) {
	if len(st.Names) != len(st.Exprs) {
		c.errors = append(c.errors, typedErr(
			"type", "arity_mismatch", "DTE0002", "arity mismatch in grouped binding",
			"assignment", len(st.Names), len(st.Exprs),
		))
	}

	_max := min(len(st.Names), len(st.Exprs))
	for i := 0; i < _max; i++ {
		name := st.Names[i]
		rk := c.kindOfExpr(st.Exprs[i])

		v, ok := c.scope.lookup(name)
		if !ok {
			c.errors = append(c.errors, fmt.Errorf("assign to undeclared variable %q", name))
			continue
		}
		if !v.mutable {
			c.errors = append(c.errors, fmt.Errorf("cannot assign to immutable variable %q", name))
			continue
		}
		if k, ok := unifyKinds(v.kind, rk); !ok {
			c.errors = append(c.errors, fmt.Errorf("type mismatch: %q is %s but assigned %s", name, v.kind, rk))
		} else if v.kind == KindUnknown {
			v.kind = k
		}
		v.written = true
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

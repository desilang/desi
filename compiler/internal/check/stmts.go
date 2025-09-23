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
				c.errors = append(c.errors, ErrWrongReturnKind(fmt.Sprintf("%s", exp), "void", "return"))
			}
			if br := top(c.blockReturned); br != nil {
				*br = true
			}
			return
		}
		got := c.kindOfExpr(st.Expr)
		if exp == KindVoid {
			c.errors = append(c.errors, ErrWrongReturnKind("void", fmt.Sprintf("%s", got), "return"))
			if br := top(c.blockReturned); br != nil {
				*br = true
			}
			return
		}
		if _, ok := unifyKinds(exp, got); !ok {
			c.errors = append(c.errors, ErrWrongReturnKind(fmt.Sprintf("%s", exp), fmt.Sprintf("%s", got), "return"))
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

	_max := _min(len(st.Binds), len(st.Values))
	for i := 0; i < _max; i++ {
		bd := st.Binds[i]
		rk := c.kindOfExpr(st.Values[i])

		declText := strings.TrimSpace(bd.Type)
		want, sname := mapTypeOrStruct(declText, c.info)

		kind := rk
		if declText != "" {
			if want == KindStruct {
				// For now: allow any RHS kind (will be refined when struct values exist).
				kind = KindStruct
			} else if want != KindUnknown {
				if k, ok := unifyKinds(want, rk); ok {
					kind = k
				} else {
					c.errors = append(c.errors, fmt.Errorf("let %q: type mismatch (declared %s, got %s)", bd.Name, want, rk))
				}
			}
		}

		// Shadowing warning
		if _, ok := c.scope.lookupLocal(bd.Name); !ok && c.scope.existsInOuter(bd.Name) {
			c.warnings = append(c.warnings, Warning{
				Code: CodeShadowedVariable(),
				Msg:  fmt.Sprintf("name %q shadows an outer binding", bd.Name),
			})
		}

		v := &varInfo{
			kind:       kindIfDeclOr(kind, want),
			mutable:    st.Mutable,
			declName:   bd.Name,
			structName: snameIfDeclOr(sname, want),
			written:    true,
		}
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

	_max := _min(len(st.Names), len(st.Exprs))
	for i := 0; i < _max; i++ {
		name := st.Names[i]
		rk := c.kindOfExpr(st.Exprs[i])

		v, ok := c.scope.lookup(name)
		if !ok {
			c.errors = append(c.errors, ErrUndefinedName(name, "assignment"))
			continue
		}
		if !v.mutable {
			c.errors = append(c.errors, ErrAssignToImmutable(name, "assignment"))
			continue
		}

		// Special-case struct variables: we only allow assigning from
		// another variable of the same struct type (very conservative).
		if v.kind == KindStruct {
			rhsStruct := c.structNameOfExpr(st.Exprs[i])
			if rhsStruct != "" && rhsStruct == v.structName {
				v.written = true
				continue
			}
			// accept unknown for now (no struct literals yet), otherwise mismatch
			if rk != KindUnknown {
				c.errors = append(c.errors, fmt.Errorf("assignment to %q: incompatible struct value", name))
			}
			v.written = true
			continue
		}

		if k, ok := unifyKinds(v.kind, rk); !ok {
			c.errors = append(c.errors, ErrTypeMismatch(fmt.Sprintf("%s", v.kind), fmt.Sprintf("%s", rk), "assignment"))
		} else if v.kind == KindUnknown {
			v.kind = k
		}
		v.written = true
	}
}

func (c *checker) structNameOfExpr(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.IdentExpr:
		if vi, ok := c.scope.lookup(v.Name); ok {
			return vi.structName
		}
	}
	return ""
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

/* ---------- small helpers for let handling ---------- */

func kindIfDeclOr(current Kind, want Kind) Kind {
	if want == KindStruct {
		return KindStruct
	}
	return current
}

func snameIfDeclOr(sname string, want Kind) string {
	if want == KindStruct {
		return sname
	}
	return ""
}

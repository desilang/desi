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
	case *ast.MatchStmt:
		c.checkMatch(st)
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

func (c *checker) checkMatch(m *ast.MatchStmt) {
	// Scrutinee must be an enum-typed identifier (Stage-1).
	k := c.kindOfExpr(m.Scrut) // also marks read if ident
	if k != KindEnum && k != KindUnknown {
		c.errors = append(c.errors, fmt.Errorf("match scrutinee must be an enum, got %s", k))
	}

	enumName := ""
	if id, ok := m.Scrut.(*ast.IdentExpr); ok {
		if vi, ok := c.scope.lookup(id.Name); ok {
			enumName = vi.structName
		}
	}
	if enumName == "" {
		// best effort: continue to check body to surface more errors
		enumName = "_"
	}

	ei, haveEnum := c.info.Enums[enumName]

	// Check each arm
	for _, arm := range m.Arms {
		varPayload, ok := "", false
		if haveEnum {
			varPayload, ok = ei.Variants[arm.Pat.Variant]
			if !ok {
				c.errors = append(c.errors, fmt.Errorf("unknown variant %q for enum %q", arm.Pat.Variant, enumName))
			}
		}
		// Payload binding rules
		noPayload := isNoneLike(varPayload)
		if noPayload && arm.Pat.Bind != "" {
			c.errors = append(c.errors, fmt.Errorf("variant %q has no payload; remove binding %q", arm.Pat.Variant, arm.Pat.Bind))
		}
		if !noPayload && arm.Pat.Bind == "" {
			// keep forgiving, but flag clearly
			c.warnings = append(c.warnings, Warning{
				Code: warnCode("warn", "unused_payload", "DW0007"),
				Msg:  fmt.Sprintf("variant %q carries a payload but arm does not bind it", arm.Pat.Variant),
			})
		}

		// Body in a child scope; if bound, define it
		c.withBlock(func() {
			if !noPayload && arm.Pat.Bind != "" {
				kind, sname := mapTypeOrStruct(varPayload, c.info)
				_ = sname // kept for future richness
				v := &varInfo{
					kind:       kind,
					mutable:    false,
					declName:   arm.Pat.Bind,
					structName: sname,
					written:    true,
				}
				if err := c.scope.define(arm.Pat.Bind, v); err != nil {
					c.errors = append(c.errors, err)
				} else {
					c.locals = append(c.locals, v)
				}
			}
			for _, s2 := range arm.Body {
				c.checkStmt(s2)
			}
		})
	}
}

func (c *checker) checkLet(st *ast.LetStmt) {
	// Arity check
	if len(st.Binds) != len(st.Values) {
		c.errors = append(c.errors, typedErr(
			"type", "arity_mismatch", "DTE0002", "arity mismatch in grouped binding",
			"let", len(st.Binds), len(st.Values),
		))
	}

	_max := _min(len(st.Binds), len(st.Values))
	for i := 0; i < _max; i++ {
		bd := st.Binds[i]
		rk := c.kindOfExpr(st.Values[i])

		declText := strings.TrimSpace(bd.Type)
		want, sname := mapTypeOrStruct(declText, c.info)

		kind := rk
		if declText != "" {
			if want == KindStruct || want == KindEnum {
				// For now: allow any RHS kind (enum/struct literals handled elsewhere later).
				kind = want
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
			structName: snameIfDeclOr(sname, want), // reused for enums too
			written:    true,
		}
		if err := c.scope.define(bd.Name, v); err != nil {
			c.errors = append(c.errors, err)
		} else {
			c.locals = append(c.locals, v)
		}
	}

	// Optional group type: no-op for now.
	if strings.TrimSpace(st.GroupType) != "" {
	}
}

func (c *checker) checkAssign(st *ast.AssignStmt) {
	// Prefer new LHS (Expr) if available; fall back to legacy Names.
	if len(st.LHS) > 0 {
		if len(st.LHS) != len(st.Exprs) {
			c.errors = append(c.errors, typedErr(
				"type", "arity_mismatch", "DTE0002", "arity mismatch in grouped binding",
				"assignment", len(st.LHS), len(st.Exprs),
			))
		}

		_max := _min(len(st.LHS), len(st.Exprs))
		for i := 0; i < _max; i++ {
			lhs := st.LHS[i]
			rk := c.kindOfExpr(st.Exprs[i])

			switch lv := lhs.(type) {
			case *ast.IdentExpr:
				v, ok := c.scope.lookup(lv.Name)
				if !ok {
					c.errors = append(c.errors, ErrUndefinedName(lv.Name, "assignment"))
					continue
				}
				if !v.mutable {
					c.errors = append(c.errors, ErrAssignToImmutable(lv.Name, "assignment"))
					continue
				}
				// Struct whole-value assignment (very conservative).
				if v.kind == KindStruct {
					rhsStruct := c.structNameOfExpr(st.Exprs[i])
					if rhsStruct != "" && rhsStruct == v.structName {
						v.written = true
						continue
					}
					if rk != KindUnknown {
						c.errors = append(c.errors, fmt.Errorf("assignment to %q: incompatible struct value", lv.Name))
					}
					v.written = true
					continue
				}
				// Enum whole-value assignment (match enum name).
				if v.kind == KindEnum {
					rhsEnum := c.enumNameOfExpr(st.Exprs[i])
					if rhsEnum != "" && rhsEnum == v.structName {
						v.written = true
						continue
					}
					if rk != KindUnknown {
						c.errors = append(c.errors, fmt.Errorf("assignment to %q: incompatible enum value", lv.Name))
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

			case *ast.FieldExpr:
				// Struct field assignment checking (unchanged)
				base, path := decomposeFieldChain(lv)
				if base == "" || len(path) == 0 {
					c.errors = append(c.errors, fmt.Errorf("unsupported assignment target"))
					continue
				}
				bv, ok := c.scope.lookup(base)
				if !ok {
					c.errors = append(c.errors, ErrUndefinedName(base, "assignment"))
					continue
				}
				if !bv.mutable {
					c.errors = append(c.errors, ErrAssignToImmutable(base, "field assignment"))
					continue
				}
				if bv.kind != KindStruct || bv.structName == "" {
					c.errors = append(c.errors, fmt.Errorf("cannot assign to field on non-struct %q", base))
					continue
				}
				current := bv.structName
				for j := 0; j < len(path)-1; j++ {
					si, ok := c.info.Structs[current]
					if !ok {
						c.errors = append(c.errors, fmt.Errorf("unknown struct type %q", current))
						current = ""
						break
					}
					ftText, ok := si.Fields[path[j]]
					if !ok {
						c.errors = append(c.errors, fmt.Errorf("unknown field %q on struct %q", path[j], current))
						current = ""
						break
					}
					k, sname := mapTypeOrStruct(ftText, c.info)
					if k != KindStruct || sname == "" {
						c.errors = append(c.errors, fmt.Errorf("field %q on %q is not a struct", path[j], current))
						current = ""
						break
					}
					current = sname
				}
				if current == "" {
					continue
				}
				si, ok := c.info.Structs[current]
				if !ok {
					c.errors = append(c.errors, fmt.Errorf("unknown struct type %q", current))
					continue
				}
				last := path[len(path)-1]
				ftText, ok := si.Fields[last]
				if !ok {
					c.errors = append(c.errors, fmt.Errorf("unknown field %q on struct %q", last, current))
					continue
				}
				want, _ := mapTypeOrStruct(ftText, c.info)
				if want != KindUnknown {
					if _, ok := unifyKinds(want, rk); !ok {
						c.errors = append(c.errors, ErrTypeMismatch(fmt.Sprintf("%s", want), fmt.Sprintf("%s", rk), "field assignment"))
					}
				}
				bv.written = true

			default:
				c.errors = append(c.errors, fmt.Errorf("unsupported assignment target"))
			}
		}
		return
	}

	// ---- Legacy path (Names) ---- (unchanged)
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

		if v.kind == KindStruct {
			rhsStruct := c.structNameOfExpr(st.Exprs[i])
			if rhsStruct != "" && rhsStruct == v.structName {
				v.written = true
				continue
			}
			if rk != KindUnknown {
				c.errors = append(c.errors, fmt.Errorf("assignment to %q: incompatible struct value", name))
			}
			v.written = true
			continue
		}
		if v.kind == KindEnum {
			rhsEnum := c.enumNameOfExpr(st.Exprs[i])
			if rhsEnum != "" && rhsEnum == v.structName {
				v.written = true
				continue
			}
			if rk != KindUnknown {
				c.errors = append(c.errors, fmt.Errorf("assignment to %q: incompatible enum value", name))
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
	if want == KindStruct || want == KindEnum {
		return want
	}
	return current
}

func snameIfDeclOr(sname string, want Kind) string {
	if want == KindStruct || want == KindEnum {
		return sname
	}
	return ""
}

// decomposeFieldChain flattens a.b.c into ("a", ["b","c"]).
func decomposeFieldChain(e *ast.FieldExpr) (string, []string) {
	var parts []string
	cur := e
	parts = append(parts, cur.Name)
	for {
		if id, ok := cur.X.(*ast.IdentExpr); ok {
			// reverse parts
			for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
				parts[i], parts[j] = parts[j], parts[i]
			}
			return id.Name, parts
		}
		if fe, ok := cur.X.(*ast.FieldExpr); ok {
			parts = append(parts, fe.Name)
			cur = fe
			continue
		}
		return "", nil
	}
}

// isNoneLike treats "", "none", or "void" as "no payload".
func isNoneLike(s string) bool {
	t := strings.TrimSpace(strings.ToLower(s))
	return t == "" || t == "none" || t == "void"
}

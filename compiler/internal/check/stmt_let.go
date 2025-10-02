package check

import (
	"fmt"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
)

func (c *checker) checkLet(st *ast.LetStmt) {
	// Arity check
	if len(st.Binds) != len(st.Values) {
		c.errors = append(c.errors, ErrTypeArityMismatch("let", len(st.Binds), len(st.Values)))
	}

	_max := _min(len(st.Binds), len(st.Values))
	for i := 0; i < _max; i++ {
		bd := st.Binds[i]
		rhs := st.Values[i]
		rk := c.kindOfExpr(rhs)

		// ---- Forbid reserved/builtins and imported names as identifiers ----
		if isReservedIdent(bd.Name) {
			c.errors = append(c.errors, ErrReservedIdentifier(bd.Name, "let binding"))
			continue
		}
		if isPreludeBuiltin(bd.Name) {
			c.errors = append(c.errors, ErrShadowBuiltin(bd.Name, "let binding"))
			continue
		}
		if c.info != nil && c.info.ImportedNames != nil && c.info.ImportedNames[bd.Name] {
			c.errors = append(c.errors, ErrImportNameConflict(bd.Name, "let binding"))
			continue
		}

		declText := strings.TrimSpace(bd.Type)
		want, snameDecl := mapTypeOrStruct(declText, c.info)

		// Infer precise struct/enum names from RHS when possible.
		inferredStruct := ""
		inferredEnum := ""
		if rk == KindStruct {
			// Known struct literal?
			if sl, ok := rhs.(*ast.StructLit); ok {
				// Unknown struct type? Report early.
				if _, exists := c.info.Structs[sl.Name]; !exists {
					c.errors = append(c.errors, fmt.Errorf("unknown struct type %q", sl.Name))
				} else {
					inferredStruct = sl.Name
				}
			} else {
				inferredStruct = c.structNameOfExpr(rhs)
			}
		} else if rk == KindEnum {
			inferredEnum = c.enumNameOfExpr(rhs)
		}

		// Choose the variable's kind (decl types win over inference when provided).
		kind := rk
		if declText != "" {
			if want == KindStruct || want == KindEnum {
				kind = want
			} else if want != KindUnknown {
				if k, ok := unifyKinds(want, rk); ok {
					kind = k
				} else {
					c.errors = append(c.errors, ErrTypeMismatch(want.String(), rk.String(), fmt.Sprintf("let %q", bd.Name)))
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

		// Decide stored struct/enum name:
		var storedSName string
		switch {
		case want == KindStruct || want == KindEnum:
			storedSName = snameDecl
		case kind == KindStruct && inferredStruct != "":
			storedSName = inferredStruct
		case kind == KindEnum && inferredEnum != "":
			storedSName = inferredEnum
		}

		v := &varInfo{
			kind:       kindIfDeclOr(kind, want),
			mutable:    st.Mutable,
			declName:   bd.Name,
			structName: storedSName, // used for both struct and enum names
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

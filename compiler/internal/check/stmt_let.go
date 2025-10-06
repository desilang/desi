package check

import (
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
)

func (c *checker) checkLet(st *ast.LetStmt) {
	// Arity check
	if len(st.Binds) != len(st.Values) {
		c.errors = append(c.errors, ErrTypeArityMismatch("let", len(st.Binds), len(st.Values)))
	}

	// Binder hygiene + forbid textual 'void' in type positions
	for _, bd := range st.Binds {
		// Identifier hygiene (span-carrying)
		if isReservedIdent(bd.Name) {
			c.errors = append(c.errors, ErrReservedIdentifierAt(bd.Span, bd.Name, "let binding"))
		}
		if isPreludeBuiltin(bd.Name) {
			c.errors = append(c.errors, ErrShadowBuiltinAt(bd.Span, bd.Name, "let binding"))
		}
		// Conflicts with imported names in this function (from-import aliases or module aliases)
		if _, ok := c.aliases[bd.Name]; ok {
			c.errors = append(c.errors, ErrImportNameConflictAt(bd.Span, bd.Name, "let binding"))
		}
		if _, ok := c.modAliases[bd.Name]; ok {
			c.errors = append(c.errors, ErrImportNameConflictAt(bd.Span, bd.Name, "let binding"))
		}

		// Forbid textual 'void' as a type
		if strings.EqualFold(strings.TrimSpace(bd.Type), "void") {
			c.errors = append(c.errors, ErrUseNoneInsteadOfVoidAt(bd.Span, "let binding type"))
		}
	}

	// Group type cannot be 'void'
	if strings.EqualFold(strings.TrimSpace(st.GroupType), "void") {
		c.errors = append(c.errors, ErrUseNoneInsteadOfVoidAt(st.Span, "let group type"))
	}

	_max := _min(len(st.Binds), len(st.Values))
	for i := 0; i < _max; i++ {
		bd := st.Binds[i]
		rhs := st.Values[i]
		rk := c.kindOfExpr(rhs)

		declText := strings.TrimSpace(bd.Type)
		want, snameDecl := mapTypeOrStruct(declText, c.info)

		// Infer precise struct/enum names from RHS when possible.
		inferredStruct := ""
		inferredEnum := ""
		if rk == KindStruct {
			// Known struct literal with a type name?
			if sl, ok := rhs.(*ast.StructLit); ok {
				if _, exists := c.info.Structs[sl.Name]; !exists {
					c.errors = append(c.errors, ErrUnknownStructTypeAt(sl.Span, sl.Name))
				} else {
					inferredStruct = sl.Name
				}
			} else {
				inferredStruct = c.structNameOfExpr(rhs)
			}
		} else if rk == KindEnum {
			inferredEnum = c.enumNameOfExpr(rhs)
		}

		// Choose final variable kind (decl wins over inference when provided).
		kind := rk
		if declText != "" {
			switch {
			case want == KindStruct || want == KindEnum:
				kind = want
			case want != KindUnknown:
				if k, ok := unifyKinds(want, rk); ok {
					kind = k
				} else {
					c.errors = append(c.errors, ErrTypeMismatch(want.String(), rk.String(), "let binding"))
				}
			}
		}

		// Shadowing warning (with outer-scope check)
		if _, ok := c.scope.lookupLocal(bd.Name); !ok && c.scope.existsInOuter(bd.Name) {
			c.warnings = append(c.warnings, Warning{
				Code: CodeShadowedVariable(),
				Msg:  "name \"" + bd.Name + "\" shadows an outer binding",
			})
		}

		// Decide stored struct/enum name for this variable
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
			kind:       kind,
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

	// Optional group type currently unused beyond 'void' check.
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

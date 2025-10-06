package check

import (
	"fmt"
	"sort"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
)

/* ---------- match checking (enums) ---------- */

func (c *checker) checkMatch(m *ast.MatchStmt) {
	// Scrutinee must be an enum-typed expression (or Unknown while editing).
	sk := c.kindOfExpr(m.Scrut)
	if sk != KindEnum && sk != KindUnknown {
		c.errors = append(c.errors,
			TypeErrorAtf(m.Span, "match_non_enum", "DTE0045", "match expects enum scrutinee",
				"%s: got %s", sk.String()))
	}

	// Try to resolve the enum name from the scrutinee expression.
	enumName := c.enumNameOfExpr(m.Scrut)
	if enumName == "" {
		enumName = c.enumNameFromExprFallback(m.Scrut)
	}

	// Build variant universe for exhaustiveness & payload typing.
	var universe map[string]struct{}
	var payloadType = make(map[string]string)
	if enumName != "" {
		if ei, ok := c.info.Enums[enumName]; ok {
			universe = make(map[string]struct{}, len(ei.Variants))
			for vname, p := range ei.Variants {
				universe[vname] = struct{}{}
				payloadType[vname] = p
			}
		}
	}

	// Track duplicates & seen
	seen := map[string]struct{}{}
	wildcardSeen := false

	for i := range m.Arms {
		arm := &m.Arms[i]

		// Wildcard arm: "_:" — no variant name, no binder, just check body.
		if arm.Pat.Variant == "_" {
			if wildcardSeen {
				c.errors = append(c.errors, ErrDuplicateMatchArmAt(arm.Span, "_"))
			}
			wildcardSeen = true
			c.withBlock(func() {
				for _, s := range arm.Body {
					c.checkStmt(s)
				}
			})
			continue
		}

		// Unknown enum? We still check arm bodies in a block for general errors.
		if enumName == "" {
			c.withBlock(func() {
				if arm.Pat.Bind != "" && arm.Pat.Bind != "_" {
					_ = c.scope.define(arm.Pat.Bind, &varInfo{
						kind:     KindUnknown,
						mutable:  false,
						declName: arm.Pat.Bind,
						written:  true,
					})
				}
				for _, s := range arm.Body {
					c.checkStmt(s)
				}
			})
			continue
		}

		// Validate variant exists
		if _, ok := universe[arm.Pat.Variant]; !ok {
			c.errors = append(c.errors, ErrUnknownEnumVariantAt(arm.Span, arm.Pat.Variant, enumName))
			// still check body
			c.withBlock(func() {
				for _, s := range arm.Body {
					c.checkStmt(s)
				}
			})
			continue
		}

		// Duplicate variant in same match?
		if _, dup := seen[arm.Pat.Variant]; dup {
			c.errors = append(c.errors, ErrDuplicateMatchArmAt(arm.Span, arm.Pat.Variant))
		}
		seen[arm.Pat.Variant] = struct{}{}

		// Payload binder typing (if any)
		pt := strings.TrimSpace(payloadType[arm.Pat.Variant])
		c.withBlock(func() {
			if !isNoneText(pt) {
				// payloadful variant
				if arm.Pat.Bind != "" && arm.Pat.Bind != "_" {
					k, sname := mapTypeOrStruct(pt, c.info)
					_ = c.scope.define(arm.Pat.Bind, &varInfo{
						kind:       k,
						structName: sname, // reused for struct-or-enum name
						mutable:    false,
						declName:   arm.Pat.Bind,
						written:    true,
					})
				}
			} else {
				// payloadless variant
				if arm.Pat.Bind != "" {
					c.errors = append(c.errors, ErrPayloadlessVariantBinderAt(arm.Span, arm.Pat.Variant, arm.Pat.Bind))
				}
			}

			for _, s := range arm.Body {
				c.checkStmt(s)
			}
		})
	}

	// Exhaustiveness warning (only if we know the enum and no wildcard)
	if enumName != "" && universe != nil && !wildcardSeen {
		missing := make([]string, 0, len(universe))
		for v := range universe {
			if _, ok := seen[v]; !ok {
				missing = append(missing, v)
			}
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			c.warnings = append(c.warnings, Warning{
				Code: warnCode("warn", "non_exhaustive_match", "DW0007"),
				Msg:  fmt.Sprintf("non-exhaustive match (in enum %s): missing %s", enumName, strings.Join(missing, ", ")),
			})
		}
	}
}

// Try to infer enum name for non-identifier scrutinee, e.g. Result.Ok(1) calls.
func (c *checker) enumNameFromExprFallback(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.CallExpr:
		// Result.Ok(1) → Callee is FieldExpr(X=Ident(Result), Name=Ok)
		if fe, ok := v.Callee.(*ast.FieldExpr); ok {
			if id, ok := fe.X.(*ast.IdentExpr); ok {
				if _, ok2 := c.info.Enums[id.Name]; ok2 {
					return id.Name
				}
			}
		}
	case *ast.FieldExpr:
		if id, ok := v.X.(*ast.IdentExpr); ok {
			if _, ok2 := c.info.Enums[id.Name]; ok2 {
				return id.Name
			}
		}
	}
	return ""
}

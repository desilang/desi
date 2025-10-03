package check

import (
	"fmt"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
)

func checkFunc(info *Info, fn *ast.FuncDecl, fromAliasMap map[string]string, modAliasMap map[string]string) ([]error, []Warning) {
	c := &checker{
		info:       info,
		fnSig:      info.Funcs[fn.Name],
		scope:      &scope{vars: map[string]*varInfo{}},
		locals:     nil,
		aliases:    fromAliasMap,
		modAliases: modAliasMap,
	}
	// Feature flags (default ON; CLI may toggle later)
	c.features.Async = true

	// params (immutable) — hygiene w/ spans + forbid collisions with imported names (aliases/modules)
	for i, p := range fn.Params {
		if isReservedIdent(p.Name) {
			c.errors = append(c.errors, ErrReservedIdentifierAt(p.Span, p.Name, "parameter"))
		}
		if isPreludeBuiltin(p.Name) {
			c.errors = append(c.errors, ErrShadowBuiltinAt(p.Span, p.Name, "parameter"))
		}
		if _, ok := c.aliases[p.Name]; ok {
			c.errors = append(c.errors, ErrImportNameConflictAt(p.Span, p.Name, "parameter"))
		}
		if _, ok := c.modAliases[p.Name]; ok {
			c.errors = append(c.errors, ErrImportNameConflictAt(p.Span, p.Name, "parameter"))
		}

		k, sname := mapTypeOrStruct(p.Type, info)
		v := &varInfo{
			kind:       k,
			mutable:    false,
			declName:   p.Name,
			structName: sname, // reused for both struct+enum
			read:       false,
			written:    true,
		}
		if err := c.scope.define(p.Name, v); err != nil {
			c.errors = append(c.errors, fmt.Errorf("parameter %d %q: %v", i, p.Name, err))
		}
		c.locals = append(c.locals, v)
	}

	c.blockReturned = push(c.blockReturned, false)
	for _, s := range fn.Body {
		c.checkStmt(s)
	}
	hasReturn := *top(c.blockReturned)
	c.blockReturned = pop(c.blockReturned)

	// Non-void fallthrough check (keep as-is)
	if fnRet := c.fnSig.Ret; fnRet != KindVoid && !hasReturn {
		if fnRet != KindFuture {
			tailExprOK := false
			if len(fn.Body) > 0 {
				if es, ok := fn.Body[len(fn.Body)-1].(*ast.ExprStmt); ok {
					tk := c.kindOfExpr(es.Expr)
					if _, ok := unifyKinds(fnRet, tk); ok {
						tailExprOK = true
					}
				}
			}
			if !tailExprOK {
				c.warnings = append(c.warnings, Warning{
					Code: warnCode("warn", "missing_explicit_return", "DW0006"),
					Msg:  fmt.Sprintf("function %q returns %s but may fall through without an explicit return", fn.Name, fnRet),
				})
			}
		}
	}

	// Unused vars/params (ignore names starting with "_")
	for _, v := range c.locals {
		if strings.HasPrefix(v.declName, "_") {
			continue
		}
		if !v.read {
			c.warnings = append(c.warnings, Warning{
				Code: warnCode("warn", "unused_variable", "DW0001"),
				Msg:  fmt.Sprintf("unused variable or parameter %q", v.declName),
			})
		}
	}

	return c.errors, c.warnings
}

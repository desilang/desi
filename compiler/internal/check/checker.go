package check

import (
	"fmt"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
)

type checker struct {
	info  *Info
	fnSig FuncSig

	scope *scope

	errors   []error
	warnings []Warning

	locals        []*varInfo
	blockReturned []bool
}

// CheckFile performs semantic checks and returns info, errors, and warnings.
func CheckFile(f *ast.File) (*Info, []error, []Warning) {
	info := &Info{Funcs: map[string]FuncSig{}}
	var errs []error
	var warns []Warning

	// collect function signatures
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if _, exists := info.Funcs[fn.Name]; exists {
			errs = append(errs, fmt.Errorf("duplicate function %q", fn.Name))
			continue
		}
		var ps []Kind
		for _, p := range fn.Params {
			ps = append(ps, mapTextType(p.Type))
		}
		info.Funcs[fn.Name] = FuncSig{Name: fn.Name, Params: ps, Ret: mapTextType(fn.Ret)}
	}

	// check bodies
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok {
			fnErrs, fnWarns := checkFunc(info, fn)
			errs = append(errs, fnErrs...)
			warns = append(warns, fnWarns...)
		}
	}
	return info, errs, warns
}

func checkFunc(info *Info, fn *ast.FuncDecl) ([]error, []Warning) {
	c := &checker{
		info:   info,
		fnSig:  info.Funcs[fn.Name],
		scope:  &scope{vars: map[string]*varInfo{}},
		locals: nil,
	}
	// params (immutable)
	for i, p := range fn.Params {
		v := &varInfo{
			kind:     mapTextType(p.Type),
			mutable:  false,
			declName: p.Name,
			read:     false,
			written:  true,
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

	// Non-void fallthrough warning (codegen synthesizes default)
	if fnRet := c.fnSig.Ret; fnRet != KindVoid && !hasReturn {
		c.warnings = append(c.warnings, Warning{
			Code: "W0006",
			Msg:  fmt.Sprintf("function %q returns %s but may fall through without an explicit return", fn.Name, fnRet),
		})
	}

	// Unused vars/params (ignore names starting with "_")
	for _, v := range c.locals {
		if strings.HasPrefix(v.declName, "_") {
			continue
		}
		if !v.read {
			c.warnings = append(c.warnings, Warning{
				Code: "W0001",
				Msg:  fmt.Sprintf("unused variable or parameter %q", v.declName),
			})
		}
	}

	return c.errors, c.warnings
}

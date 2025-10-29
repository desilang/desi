package check

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/resolve"
	"github.com/desilang/desi/compiler/internal/types"
)

// ---------- public API ----------

type Result struct {
	Diags []diag.Diagnostic
	Info  *Info
}

// CheckWithLoader resolves + checks using the provided module loader.
func CheckWithLoader(mod *ast.Module, ldr resolve.Loader) *Result {
	res := &Result{Info: NewInfo()}

	// 1) Resolve imports up-front (Phase-1).
	rdiags, rinfo := resolve.Resolve(mod, ldr)
	res.Diags = append(res.Diags, rdiags...)

	// 2) Create a real top scope and inject resolver-provided bindings.
	top := NewScope(nil)
	injectImports(top, rinfo)

	// 3) Walk module for simple checks (M4/M5 level).
	c := &checker{
		info:  res.Info,
		diags: nil,
		// Child of top so imported names are visible but we keep our own defs tidy.
		scope: NewScope(top),
	}

	// Pass 1: collect function declarations (for exact-match overloading).
	for _, d := range mod.Decls {
		switch dd := d.(type) {
		case *ast.FuncDecl:
			c.collectFunc(dd)
		case *ast.ClassDecl:
			for _, m := range dd.Methods {
				c.collectFunc(m)
			}
		}
	}

	// Pass 2: check bodies.
	for _, d := range mod.Decls {
		switch dd := d.(type) {
		case *ast.FuncDecl:
			c.checkFunc(dd)
		case *ast.ClassDecl:
			for _, m := range dd.Methods {
				c.checkFunc(m)
			}
		}
	}

	// Merge checker diagnostics
	res.Diags = append(res.Diags, c.diags...)
	return res
}

// Check keeps old behavior (mem loader) for tests that don’t care about FS roots.
func Check(mod *ast.Module) *Result { return CheckWithLoader(mod, resolve.NewMemLoader(nil)) }

// ---------- internals ----------

type checker struct {
	info       *Info
	diags      []diag.Diagnostic
	scope      *Scope
	curFuncRet types.T
}

func (c *checker) add(diag diag.Diagnostic) { c.diags = append(c.diags, diag) }

func (c *checker) collectFunc(fd *ast.FuncDecl) {
	name := fd.Name.Name
	// Build function type from parameter annotations (basic names only for M4/5).
	params := make([]types.T, len(fd.Params))
	for i, p := range fd.Params {
		if p.Type != nil {
			if t, ok := types.FromName(p.Type.Name); ok {
				params[i] = t
			}
		}
	}
	var ret types.T = types.None
	if fd.RetType != nil {
		if t, ok := types.FromName(fd.RetType.Name); ok {
			ret = t
		}
	}
	sig := types.FuncOf(params, ret)

	set := c.info.Funcs[name]
	if set == nil {
		set = &OverloadSet{Name: name}
		c.info.Funcs[name] = set
	}
	set.Add(&FuncCand{Decl: fd, Type: sig})

	// Also bind the function name in the current scope for id resolution.
	_ = c.scope.Define(&Symbol{Name: name, Kind: SymFunc, Type: sig, Node: fd})
}

func (c *checker) checkFunc(fd *ast.FuncDecl) {
	// New scope for parameters and locals
	savedScope := c.scope
	c.scope = NewScope(c.scope)
	defer func() { c.scope = savedScope }()

	// Bind params
	for i := range fd.Params {
		p := fd.Params[i]
		var pt types.T
		if p.Type != nil {
			pt, _ = types.FromName(p.Type.Name)
		}
		_ = c.scope.Define(&Symbol{
			Name: p.Name.Name, Kind: SymParam, Type: pt, Node: &fd.Params[i].Name,
		})
	}

	// Declared return type (if any)
	c.curFuncRet = types.None
	if fd.RetType != nil {
		if t, ok := types.FromName(fd.RetType.Name); ok {
			c.curFuncRet = t
		}
	}

	// Body
	if fd.Body != nil {
		c.checkBlock(fd.Body)
	}
}

func (c *checker) checkBlock(b *ast.Block) {
	if b == nil {
		return
	}
	for _, s := range b.Stmts {
		c.checkStmt(s)
	}
}

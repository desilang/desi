package check

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/resolve"
	"github.com/desilang/desi/compiler/internal/types"
)

// ---------- public API ----------

// Result is a structured return used by the CLI.
type Result struct {
	Diags []diag.Diagnostic
	Info  *Info
}

// Check keeps the legacy/public surface that tests expect: (diags, info).
func Check(mod *ast.Module) ([]diag.Diagnostic, *Info) {
	res := CheckWithLoader(mod, resolve.NewMemLoader(nil))
	return res.Diags, res.Info
}

// CheckWithLoader resolves + checks using the provided loader (used by the CLI).
func CheckWithLoader(mod *ast.Module, ldr resolve.Loader) *Result {
	res := &Result{Info: NewInfo()}

	// Set up unused-import tracker (bindings from AST); usage is counted post-check.
	ut := newUsageTracker(mod)

	// 1) Resolve imports up-front (Phase-2 brings typed exports in rinfo).
	rdiags, rinfo := resolve.Resolve(mod, ldr)
	res.Diags = append(res.Diags, rdiags...)

	// 2) Create the top scope and inject bindings.
	top := NewScope(nil)
	// Make prelude callables visible as identifiers in the top scope (avoid DTE0001).
	injectPreludeIntoScope(top, res.Info)
	// Resolver-provided imports (modules and from-items) become top-scope names.
	injectImports(top, rinfo)

	// 2b) Bridge real signatures for from-import aliases (Phase-2).
	PopulateImportedFuncSigs(mod, res.Info, rinfo)

	// 3) Walk module: collect functions first, then check bodies.
	c := &checker{
		info:  res.Info,
		scope: NewScope(top), // child of top so imported names & prelude are visible
	}

	// Pass 1: collect functions for overload sets.
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

	// Merge checker diagnostics.
	res.Diags = append(res.Diags, c.diags...)

	// 4) After we know which identifiers resolved to which symbols,
	//    compute unused-import warnings and append them.
	ut.countUsesFromIdents(res.Info.Idents)
	ut.countUsesFromTypes(res.Info)
	res.Diags = append(res.Diags, ut.emitUnusedDiags()...)

	return res
}

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

	// Build function type from parameter annotations (basic names only for M4/M5).
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

	// Bind the function name in the current scope for call resolution.
	_ = c.scope.Define(&Symbol{Name: name, Kind: SymFunc, Type: sig, Node: fd})
}

func (c *checker) checkFunc(fd *ast.FuncDecl) {
	// New scope for parameters and locals.
	saved := c.scope
	c.scope = NewScope(c.scope)
	defer func() { c.scope = saved }()

	// Bind params.
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

	// Declared return type (if any).
	c.curFuncRet = types.None
	if fd.RetType != nil {
		if t, ok := types.FromName(fd.RetType.Name); ok {
			c.curFuncRet = t
		}
	}

	// Body.
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

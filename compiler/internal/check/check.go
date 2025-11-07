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

	// Bridge resolver info and pre-computed import-name -> module-path map for qualified calls.
	res.Info.R = rinfo
	res.Info.ImportPaths = computeImportPaths(mod)

	// 2) Create the top scope and inject bindings.
	top := NewScope(nil)
	// Make prelude callables visible as identifiers in the top scope (avoid DTE0001).
	injectPreludeIntoScope(top, res.Info)
	// Resolver-provided imports (modules and from-items) become top-scope names.
	injectImports(top, rinfo)

	// Phase-2: build exact signatures for 'from … import …' into Info.Funcs.
	PopulateImportedFuncSigs(mod, res.Info, rinfo)

	// 3) Walk module: collect functions first, then check bodies.
	c := &checker{
		info:  res.Info,
		scope: NewScope(top), // child of top so imported names & prelude are visible
	}
	// Remember the module (file-level) scope for anti-shadowing checks.
	c.moduleScope = c.scope

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

type checker struct {
	info        *Info
	diags       []diag.Diagnostic
	scope       *Scope
	moduleScope *Scope
	curFuncRet  types.T
	moved       MoveSet
	unsafeDepth int
}

func (c *checker) add(diag diag.Diagnostic) { c.diags = append(c.diags, diag) }

func (c *checker) collectFunc(fd *ast.FuncDecl) {
	name := fd.Name.Name

	// Top-level shadowing of builtin names is not allowed.
	c.forbidBuiltinShadowing(name, fd.Name.Span)

	// Build function type from parameter annotations (surface forms allowed).
	params := make([]types.T, len(fd.Params))
	for i, p := range fd.Params {
		if p.Type != nil {
			if tt := surfaceToType(p.Type.Name); tt != nil {
				params[i] = tt
			}
		}
	}
	var ret types.T = types.None
	if fd.RetType != nil {
		if tt := surfaceToType(fd.RetType.Name); tt != nil {
			ret = tt
		}
	}
	sig := types.FuncOf(params, ret)

	set := c.info.Funcs[name]
	if set == nil {
		set = &OverloadSet{Name: name}
		c.info.Funcs[name] = set
	}
	// Collect param modes from the declaration.
	modes := make([]ast.ParamMode, len(fd.Params))
	for i := range fd.Params {
		modes[i] = fd.Params[i].Mode
	}
	set.Add(&FuncCand{
		Decl:   fd,
		Type:   sig,
		Modes:  modes,
		Extern: isExternDecl(fd), // <-- critical: mark local @extern functions
	})

	// Bind the function name in the current scope for call resolution.
	_ = c.scope.Define(&Symbol{Name: name, Kind: SymFunc, Type: sig, Node: fd})
}

func (c *checker) checkFunc(fd *ast.FuncDecl) {
	// New scope for parameters and locals.
	saved := c.scope
	c.scope = NewScope(c.scope)
	defer func() { c.scope = saved }()

	// Reset per-function move-tracking state (our local tracker)
	c.moved = MoveSet{}
	if c.info != nil {
		c.info.Moved = make(map[string]diag.Span)
	}

	// Bind params.
	for i := range fd.Params {
		p := fd.Params[i]
		var pt types.T
		if p.Type != nil {
			pt = surfaceToType(p.Type.Name)
		}
		_ = c.scope.Define(&Symbol{
			Name: p.Name.Name, Kind: SymParam, Type: pt, Node: &fd.Params[i].Name,
		})
	}

	// Declared return type (if any).
	c.curFuncRet = types.None
	if fd.RetType != nil {
		if tt := surfaceToType(fd.RetType.Name); tt != nil {
			c.curFuncRet = tt
		}
	}

	// M6-C: callee-side borrow rule on async functions.
	c.checkAsyncInoutAwait(fd)

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

// isExternDecl reports whether a function has an @extern decorator (any args).
func isExternDecl(fd *ast.FuncDecl) bool {
	if fd == nil {
		return false
	}
	for _, dec := range fd.Decorators {
		if dec != nil && dec.Name.Name == "extern" {
			return true
		}
	}
	return false
}

// forbidBuiltinShadowing emits DPL0001 if a top-level binding redefines a builtin.
func (c *checker) forbidBuiltinShadowing(name string, sp diag.Span) {
	if c == nil || c.scope == nil {
		return
	}
	// Only enforce at module scope (top-level bindings).
	if c.scope == c.moduleScope && isPreludeBuiltinName(name) {
		c.add(diagAt("DPL0001", sp, "cannot shadow builtin: "+name))
	}
}

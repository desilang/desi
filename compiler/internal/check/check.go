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

// CheckWithLoader runs resolve + type checking using a provided loader.
func CheckWithLoader(mod *ast.Module, ldr resolve.Loader) *Result {
	res := &Result{Info: NewInfo()}

	// Resolve imports for this module with the provided loader.
	rdiags, rinfo := resolve.Resolve(mod, ldr)
	res.Diags = append(res.Diags, rdiags...)

	// Bring resolved imports/aliases into the top scope.
	if res.Info.top == nil {
		res.Info.top = NewTopScope()
	}
	res.Diags = append(res.Diags, injectImports(res.Info.top, rinfo)...)

	// Ensure from-import aliases are recognized as callables to avoid DTE0001.
	// (We don't know their types yet in Phase-1; this prevents "undefined function: <alias>".)
	for _, fi := range rinfo.FromItems {
		name := fi.Alias
		if name == "" {
			name = fi.Name
		}
		if _, ok := res.Info.Funcs[name]; !ok {
			res.Info.Funcs[name] = &OverloadSet{Name: name}
		}
	}

	// Run the checker.
	c := &checker{
		info:       res.Info,
		diags:      &res.Diags,
		scope:      res.Info.top,
		curFuncRet: nil,
	}
	c.checkModule(mod)

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

package check

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/types"
)

type checker struct {
	info       *Info
	diags      []diag.Diagnostic
	scope      *Scope
	curFuncRet types.T
}

func Check(mod *ast.Module) ([]diag.Diagnostic, *Info) {
	c := &checker{
		info:  NewInfo(),
		scope: NewScope(nil),
	}
	// Pass 1: collect function declarations for overload resolution.
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
	return c.diags, c.info
}

func (c *checker) add(diag diag.Diagnostic) { c.diags = append(c.diags, diag) }

func (c *checker) collectFunc(fd *ast.FuncDecl) {
	name := fd.Name.Name
	// Build function type from parameter annotations (basic names only for M4).
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

	// Also bind the function name in the top-level scope for callee-id resolution.
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
		// Attach the symbol to the param's identifier node (which implements ast.Node).
		_ = c.scope.Define(&Symbol{
			Name: p.Name.Name, Kind: SymParam, Type: pt, Node: &fd.Params[i].Name,
		})
	}

	// Set declared return type
	c.curFuncRet = types.None
	if fd.RetType != nil {
		if t, ok := types.FromName(fd.RetType.Name); ok {
			c.curFuncRet = t
		}
	}

	// Check body (if present)
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

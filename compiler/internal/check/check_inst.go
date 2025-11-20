package check

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/types"
)

// checkTypeCall handles T(...) where T is a type (struct or class).
func (c *checker) checkTypeCall(call *ast.CallExpr, sym *Symbol) types.T {
	switch d := sym.Node.(type) {
	case *ast.StructDecl:
		return c.checkStructInit(call, d)
	case *ast.ClassDecl:
		return c.checkClassInit(call, d)
	default:
		c.add(diagAt("DTE0105", call.Callee.SpanOf(), "type is not instantiable"))
		return nil
	}
}

func (c *checker) checkStructInit(call *ast.CallExpr, d *ast.StructDecl) types.T {
	// Helper to map fields.
	fields := make(map[string]*ast.FieldDecl)
	for _, f := range d.Fields {
		fields[f.Name.Name] = f
	}

	// Check args.
	seen := make(map[string]bool)
	for _, arg := range call.ArgNodes {
		if arg.Name == nil {
			c.add(diagAt("DTE0106", arg.Expr.SpanOf(), "struct initialization requires named arguments"))
			continue
		}
		name := arg.Name.Name
		if seen[name] {
			c.add(diagAt("DTE0107", arg.Name.Span, "duplicate argument '"+name+"'"))
			continue
		}
		seen[name] = true

		f, ok := fields[name]
		if !ok {
			c.add(diagAt("DTE0108", arg.Name.Span, "unknown field '"+name+"' in struct '"+d.Name.Name+"'"))
			continue
		}

		// Check type.
		valT := c.typ(arg.Expr)
		var fieldT types.T = types.None // Default
		if f.Type != nil {
			if t, ok := types.FromName(f.Type.Name); ok {
				fieldT = t
			}
		}

		if !types.Assignable(fieldT, valT) {
			c.add(diagAt("DTE0104", arg.Expr.SpanOf(), "field '"+name+"' expects type "+fieldT.String()))
		}
	}

	// Check missing fields.
	for _, f := range d.Fields {
		if !seen[f.Name.Name] {
			c.add(diagAt("DTE0109", call.Span, "missing field '"+f.Name.Name+"' in struct initialization"))
		}
	}

	// Return the struct instance type.
	return types.Basic(d.Name.Name, types.StructKind)
}

func (c *checker) checkClassInit(call *ast.CallExpr, d *ast.ClassDecl) types.T {
	// Return class instance type.
	// TODO: Check constructor arguments if __init__ exists.
	return types.Basic(d.Name.Name, types.ClassKind)
}

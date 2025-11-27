package check

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/types"
)

// checkTypeCall handles T(...) where T is a type (struct, class, or enum variant).
func (c *checker) checkTypeCall(call *ast.CallExpr, sym *Symbol) types.T {
	var t types.T
	switch d := sym.Node.(type) {
	case *ast.StructDecl:
		t = c.checkStructInit(call, d)
	case *ast.ClassDecl:
		t = c.checkClassInit(call, d)
	case *ast.EnumDecl:
		// For enum, T() is not valid - must use T.Variant()
		// This case shouldn't normally be hit since enums use T.Variant() syntax
		c.add(diagAt("DTE0105", call.Callee.SpanOf(), "enum types must be constructed via T.Variant() syntax"))
		return nil
	default:
		c.add(diagAt("DTE0105", call.Callee.SpanOf(), "type is not instantiable"))
		return nil
	}
	// Store the type for the call expression
	c.info.Types[call] = t
	return t
}

func (c *checker) checkStructInit(call *ast.CallExpr, d *ast.StructDecl) types.T {
	// Look up the actual struct type from the symbol table
	sym := c.scope.Lookup(d.Name.Name)
	var st *types.Struct
	if sym != nil && sym.Type != nil {
		st, _ = sym.Type.(*types.Struct)
	}

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
		var fieldT types.T = types.None

		// Get resolved field type from struct definition
		if st != nil {
			for _, sf := range st.Fields {
				if sf.Name == name {
					fieldT = sf.Type
					break
				}
			}
		} else {
			// Fallback (shouldn't happen if struct was collected)
			if f.Type != nil {
				if t, ok := types.FromName(f.Type.Name); ok {
					fieldT = t
				}
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
	if st != nil {
		return st
	}
	// Fallback (shouldn't happen if checkStruct ran)
	return types.Basic(d.Name.Name, types.StructKind)
}

func (c *checker) checkClassInit(call *ast.CallExpr, d *ast.ClassDecl) types.T {
	// Return class instance type.
	// TODO: Check constructor arguments if __init__ exists.
	return types.Basic(d.Name.Name, types.ClassKind)
}

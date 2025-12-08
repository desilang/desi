package check

import (
	"fmt"

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

	// M15: Generic Type Inference
	// If the struct is generic, we need to infer type arguments from the provided fields.
	var inferred map[string]types.T
	if st != nil && len(st.TypeParams) > 0 {
		inferred = make(map[string]types.T)
	}

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

		// Try to unify field type (pattern) with value type (concrete) to infer type params
		if inferred != nil {
			_ = unify(fieldT, valT, inferred)
			// Substitute known type params into fieldT for checking
			fieldT = substitute(fieldT, inferred)
		}

		if !types.Assignable(fieldT, valT) {
			c.add(diagAt("DTE0104", arg.Expr.SpanOf(), "field '"+name+"' expects type "+fieldT.String()))
		}

		// Mark as moved if not a copy type
		if !isCopyType(valT) {
			if name, ok := c.baseLvalue(arg.Expr); ok {
				c.moved.mark(name, arg.Expr.SpanOf())
			}
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
		// If generic, return instantiated Generic type
		if len(st.TypeParams) > 0 {
			var args []types.T
			for _, tp := range st.TypeParams {
				if t, ok := inferred[tp.Name]; ok {
					args = append(args, t)
				} else {
					c.add(diagAt("DTE0110", call.Span, "cannot infer type parameter '"+tp.Name+"'"))
					args = append(args, types.Any) // Fallback
				}
			}
			return &types.Generic{Base: st, Args: args}
		}
		return st
	}
	// Fallback (shouldn't happen if checkStruct ran)
	return types.Basic(d.Name.Name, types.StructKind)
}

func (c *checker) checkClassInit(call *ast.CallExpr, d *ast.ClassDecl) types.T {
	// Look up the class type
	sym := c.scope.Lookup(d.Name.Name)
	if sym == nil {
		return types.Any
	}

	cls, ok := sym.Type.(*types.Class)
	if !ok {
		return types.Any
	}

	// POLICY: Prevent instantiation of abstract classes
	if cls.IsAbstract {
		c.add(diagAt("DCL0007", call.Span, fmt.Sprintf("cannot instantiate abstract class '%s'", cls.Name)))
		return types.Any
	}

	// POLICY: Check for __new__ dunder (Constructors)
	if len(cls.Constructors) > 0 {
		// Overload resolution for __new__
		var bestCand *types.Func

		// Simple score-based resolution:
		// 0: exact match
		// -1: no match

		for _, ctor := range cls.Constructors {
			// __new__ methods have an implicit self parameter as the first param (injected by check_type.go)
			// Callers do NOT pass self explicitly, so we need to skip it in arity and type checks
			params := ctor.Params
			if len(params) > 0 {
				// Check if first param is the class type (i.e., self)
				if _, ok := params[0].(*types.Class); ok {
					params = params[1:] // Skip implicit self
				}
			}
			numExpectedArgs := len(params)

			// Check arity
			if len(call.Args) != numExpectedArgs {
				continue
			}

			// Check types
			match := true
			for i, arg := range call.Args {
				argType := c.typ(arg)
				if !types.Assignable(params[i], argType) {
					match = false
					break
				}
			}

			if match {
				// Found a match!
				bestCand = ctor
				break
			}
		}

		if bestCand == nil {
			c.add(diagAt("DTE0046", call.Span, fmt.Sprintf("no matching constructor for %s with %d arguments", cls.Name, len(call.Args))))
			return types.Any
		}

		// Return class type (or generic instance for generic classes)
		if len(d.TypeParams) > 0 {
			// TODO: Infer type arguments from call arguments
			return &types.Generic{Base: cls, Args: nil}
		}
		return cls

	} else {
		// POLICY: No __new__ = implicit zero-arg constructor only
		if len(call.Args) > 0 {
			c.add(diagAt("DTE0046", call.Span, fmt.Sprintf("class %s has no __new__ and only accepts zero arguments", cls.Name)))
			return types.Any
		}

		// For generic classes, infer type arguments from expected type
		if len(d.TypeParams) > 0 {
			// Check if we have an expected type from context (e.g., let x: Box<int> = Box())
			if c.expected != nil {
				// Try to match expected type with class
				if gen, ok := c.expected.(*types.Generic); ok {
					if baseCls, ok := gen.Base.(*types.Class); ok && baseCls.Name == cls.Name {
						// Expected type is the same generic class with type args
						// Record this instantiation for monomorphization
						c.info.ClassInstantiations[cls.Name] = append(c.info.ClassInstantiations[cls.Name], gen)
						return c.expected
					}
				}
			}
			// No expected type or mismatch: return Generic with nil args (error case)
			c.add(diagAt("DTE0110", call.Span, fmt.Sprintf("cannot infer type parameters for %s; explicit type annotation required", cls.Name)))
			return &types.Generic{Base: cls, Args: nil}
		}

		return cls
	}
}

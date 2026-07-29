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
		// Fallback: if checkClassInit returned types.Any (lookup failed for nested class),
		// use the class type from sym directly since we already have it
		if t == types.Any && sym.Type != nil {
			if cls, ok := sym.Type.(*types.Class); ok {
				// For zero-arg constructors without __new__, just return the class type
				if len(call.Args) == 0 {
					t = cls
				}
			}
		}
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

	// Detect arg style: all-positional, all-named, or mixed.
	hasPositional := false
	hasNamed := false
	for _, arg := range call.ArgNodes {
		if arg.Name == nil {
			hasPositional = true
		} else {
			hasNamed = true
		}
	}

	if hasPositional && hasNamed {
		c.add(diagAt("DTE0106", call.Span, "struct initialization cannot mix positional and named arguments"))
	} else if hasPositional {
		// --- Positional mode: map args to fields by declaration order ---
		if len(call.ArgNodes) != len(d.Fields) {
			c.add(diagAt("DTE0109", call.Span,
				fmt.Sprintf("struct '%s' has %d fields but got %d arguments", d.Name.Name, len(d.Fields), len(call.ArgNodes))))
		} else {
			for i, arg := range call.ArgNodes {
				f := d.Fields[i]
				name := f.Name.Name
				seen[name] = true

				valT := c.typ(arg.Expr)
				var fieldT types.T = types.None

				if st != nil {
					for _, sf := range st.Fields {
						if sf.Name == name {
							fieldT = sf.Type
							break
						}
					}
				} else if f.Type != nil {
					if t, ok := types.FromName(f.Type.Name); ok {
						fieldT = t
					}
				}

				if inferred != nil {
					_ = unify(fieldT, valT, inferred)
					fieldT = substitute(fieldT, inferred)
				}

				if !types.Assignable(fieldT, valT) {
					c.add(diagAt("DTE0104", arg.Expr.SpanOf(), "field '"+name+"' expects type "+fieldT.String()))
				}

				if !isCopyType(valT) {
					if lv, ok := c.baseLvalue(arg.Expr); ok {
						c.moved.mark(lv, arg.Expr.SpanOf())
					}
				}
			}
		}
	} else {
		// --- Named mode: original behavior ---
		for _, arg := range call.ArgNodes {
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

			valT := c.typ(arg.Expr)
			var fieldT types.T = types.None

			if st != nil {
				for _, sf := range st.Fields {
					if sf.Name == name {
						fieldT = sf.Type
						break
					}
				}
			} else {
				if f.Type != nil {
					if t, ok := types.FromName(f.Type.Name); ok {
						fieldT = t
					}
				}
			}

			if inferred != nil {
				_ = unify(fieldT, valT, inferred)
				fieldT = substitute(fieldT, inferred)
			}

			if !types.Assignable(fieldT, valT) {
				c.add(diagAt("DTE0104", arg.Expr.SpanOf(), "field '"+name+"' expects type "+fieldT.String()))
			}

			if !isCopyType(valT) {
				if lv, ok := c.baseLvalue(arg.Expr); ok {
					c.moved.mark(lv, arg.Expr.SpanOf())
				}
			}
		}

		// Check missing fields (only in named mode — positional checks arity above).
		for _, f := range d.Fields {
			if !seen[f.Name.Name] {
				c.add(diagAt("DTE0109", call.Span, "missing field '"+f.Name.Name+"' in struct initialization"))
			}
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
					c.add(diagAt("DTE0113", call.Span, "cannot infer type parameter '"+tp.Name+"'"))
					args = append(args, types.Any) // Fallback
				}
			}
			// Validate bounds on inferred type arguments
			c.validateGenericBounds(st, args, call.Span)
			return &types.Generic{Base: st, Args: args}
		}
		return st
	}
	// Fallback (shouldn't happen if checkStruct ran)
	return types.Basic(d.Name.Name, types.StructKind)
}

// explicitTypeArgs resolves the turbofish type arguments on a constructor call,
// returning nil when there are none. A count that does not match the class's
// type parameters is reported and discarded, so a mistake surfaces here rather
// than as a confusing inference failure further on.
func (c *checker) explicitTypeArgs(call *ast.CallExpr, d *ast.ClassDecl, cls *types.Class) []types.T {
	if len(call.TypeArgs) == 0 {
		return nil
	}
	if len(d.TypeParams) == 0 {
		c.add(diagAt("DTE0114", call.Span,
			fmt.Sprintf("class %s is not generic and takes no type arguments", cls.Name)))
		return nil
	}
	if len(call.TypeArgs) != len(d.TypeParams) {
		c.add(diagAt("DTE0114", call.Span,
			fmt.Sprintf("class %s takes %d type argument(s), got %d",
				cls.Name, len(d.TypeParams), len(call.TypeArgs))))
		return nil
	}
	out := make([]types.T, 0, len(call.TypeArgs))
	for _, ta := range call.TypeArgs {
		rt := c.resolveType(ta)
		if rt == nil {
			return nil // resolveType already reported what it could not resolve
		}
		out = append(out, rt)
	}
	return out
}

func (c *checker) checkClassInit(call *ast.CallExpr, d *ast.ClassDecl) types.T {
	var cls *types.Class

	// Try scope lookup first (for top-level classes)
	sym := c.scope.Lookup(d.Name.Name)
	if sym != nil {
		cls, _ = sym.Type.(*types.Class)
	}

	// Fallback to c.info.Types for nested classes
	if cls == nil {
		if t, ok := c.info.Types[d]; ok {
			cls, _ = t.(*types.Class)
		}
	}

	if cls == nil {
		return types.Any
	}

	// POLICY: Prevent instantiation of abstract classes
	if cls.IsAbstract {
		c.add(diagAt("DCL0007", call.Span, fmt.Sprintf("cannot instantiate abstract class '%s'", cls.Name)))
		return types.Any
	}

	// Explicit type arguments from turbofish: `Stack::<int>()`.
	//
	// Inference reads the type parameters off the constructor's arguments, so a
	// constructor that mentions T nowhere — `def __new__(self)` on a
	// `Stack<T>` — leaves nothing to infer from, and the class could not be
	// constructed at all. Stating them explicitly is the answer, and turbofish
	// already parses here; it was simply never read.
	explicitArgs := c.explicitTypeArgs(call, d, cls)

	// POLICY: Check for __new__ dunder (Constructors)
	if len(cls.Constructors) > 0 {
		// Overload resolution for __new__
		var bestCand *types.Func
		var inferred map[string]types.T // For type parameter inference

		// Initialize inference map for generic classes
		if len(d.TypeParams) > 0 {
			inferred = make(map[string]types.T)
		}

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

			// Reset inferred map for each constructor candidate
			if len(d.TypeParams) > 0 {
				inferred = make(map[string]types.T)
			}

			// Check types (with unification for generic classes)
			match := true
			for i, arg := range call.Args {
				argType := c.typ(arg)
				paramType := params[i]

				// For generic classes, try to unify param type with arg type
				if len(d.TypeParams) > 0 {
					if !unify(paramType, argType, inferred) {
						// Unification failed - try with substituted param type
						substituted := substitute(paramType, inferred)
						if !types.Assignable(substituted, argType) {
							match = false
							break
						}
					}
				} else {
					// Non-generic class: direct assignability check
					if !types.Assignable(paramType, argType) {
						match = false
						break
					}
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
			// Infer type arguments from the inferred map
			var args []types.T
			for i, tp := range d.TypeParams {
				// Turbofish wins: the programmer said what T is, so there is
				// nothing left to infer and nothing to disagree about.
				if i < len(explicitArgs) {
					args = append(args, explicitArgs[i])
					continue
				}
				if t, ok := inferred[tp.Name.Name]; ok {
					args = append(args, t)
				} else {
					c.add(diagAt("DTE0113", call.Span, "cannot infer type parameter '"+tp.Name.Name+"'"))
					args = append(args, types.Any)
				}
			}
			// Validate bounds on inferred type arguments
			c.validateGenericBounds(cls, args, call.Span)
			gen := &types.Generic{Base: cls, Args: args}
			// Record this instantiation for monomorphization
			c.info.ClassInstantiations[cls.Name] = append(c.info.ClassInstantiations[cls.Name], gen)
			return gen
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
			// Turbofish first — it says what the annotation otherwise would.
			if len(explicitArgs) > 0 {
				gen := &types.Generic{Base: cls, Args: explicitArgs}
				c.validateGenericBounds(cls, explicitArgs, call.Span)
				c.info.ClassInstantiations[cls.Name] = append(c.info.ClassInstantiations[cls.Name], gen)
				return gen
			}
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
			c.add(diagAt("DTE0113", call.Span, fmt.Sprintf("cannot infer type parameters for %s; explicit type annotation required", cls.Name)))
			return &types.Generic{Base: cls, Args: nil}
		}

		return cls
	}
}

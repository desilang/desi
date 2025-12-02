package check

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/types"
)

func (c *checker) typCall(call *ast.CallExpr) types.T {
	// --- Case 0: direct call of a lambda: (lambda ...)(args)
	if l, ok := call.Callee.(*ast.LambdaExpr); ok {
		_ = c.typ(l)
		// Lambdas are positional-only in this milestone.
		if len(call.Args) != len(l.Params) {
			c.add(diagAt("DTE0046", l.Span, "arity mismatch: wrong number of arguments"))
			return nil
		}
		for i := range l.Params {
			var pt types.T
			if l.Params[i].Type != nil {
				if t, ok := types.FromName(l.Params[i].Type.Name); ok {
					pt = t
				}
			}
			at := c.typ(call.Args[i])
			if pt == nil || at == nil || !types.Equal(pt, at) {
				c.add(diagAt("DTE0104", call.Span, "argument type mismatch"))
				return nil
			}
		}
		if ft, ok := c.info.Types[l].(*types.Func); ok {
			c.info.Types[call] = ft.Ret
			return ft.Ret
		}
		return nil
	}

	// Common helpers
	argsNodes := callArgs(call)
	hasNamed := false
	for _, a := range argsNodes {
		if a.Name != nil {
			hasNamed = true
			break
		}
	}

	// --- Case 1: module-qualified call: mod.fn(...)
	if fe, ok := call.Callee.(*ast.FieldExpr); ok {
		if set, base, isImport := c.moduleQualifiedOverloadSet(fe); isImport {
			if !hasNamed {
				// Legacy positional path
				args := make([]types.T, len(argsNodes))
				for i, a := range argsNodes {
					args[i] = c.typ(a.Expr)
				}
				if set == nil || len(set.Cands) == 0 {
					c.add(diagAt("DME0003", fe.Name.Span, base.Name+" has no exported '"+fe.Name.Name+"'"))
					return nil
				}
				arityCands := filterByArity(set.Cands, len(args))
				if len(arityCands) == 0 {
					c.add(diagAt("DTE0046", fe.Name.Span, "arity mismatch: wrong number of arguments"))
					return nil
				}
				exact := filterExactByTypes(arityCands, args)
				switch len(exact) {
				case 1:
					chosen := exact[0]
					if chosen.Extern && c.unsafeDepth == 0 {
						c.add(diagAt("DFI0003", call.Callee.SpanOf(), ""))
					}
					// Synthesize positional Args for borrow/move enforcement
					tmp := *call
					tmp.Args = make([]ast.Expr, len(argsNodes))
					for i, a := range argsNodes {
						tmp.Args[i] = a.Expr
					}
					c.markMovesFromCall(chosen, &tmp, args)
					c.enforceCallsiteBorrow(chosen, &tmp)

					ret := chosen.Type.Ret
					if chosen.Decl != nil && chosen.Decl.Async {
						ret = types.FutureOf(ret)
					}
					c.info.Types[call] = ret
					return ret
				case 0:
					c.add(diagAt("DTE0101", fe.Name.Span, "no matching overload"))
					return nil
				default:
					c.add(diagAt("DTE0102", fe.Name.Span, "ambiguous overload"))
					return nil
				}
			}

			// Named-args path (per-candidate mapping) - only execute if there are named args
			if hasNamed {
				var exact []*FuncCand
				for _, cand := range set.Cands {
					vec, ok := c.canonicalizeForCandidate(cand, argsNodes)
					if !ok {
						continue
					}
					if typesMatchExactly(cand.Type.Params, vec, cand.Type.Variadic) {
						exact = append(exact, cand)
					}
				}
				switch len(exact) {
				case 1:
					chosen := exact[0]
					if chosen.Extern && c.unsafeDepth == 0 {
						c.add(diagAt("DFI0003", call.Callee.SpanOf(), ""))
					}
					// Build positional Args vector aligned to params for borrow/move
					tmp := *call
					vecE := make([]ast.Expr, len(chosen.Type.Params))
					// Map names -> indices
					pnames := c.paramNamesForCand(chosen)
					name2idx := map[string]int{}
					for i, nm := range pnames {
						if nm != "" {
							name2idx[nm] = i
						}
					}
					// Fill leading positionals
					pos := 0
					for _, an := range argsNodes {
						if an.Name == nil {
							if pos < len(vecE) {
								vecE[pos] = an.Expr
								pos++
							}
						}
					}
					// Fill named
					for _, an := range argsNodes {
						if an.Name != nil {
							if idx, ok := name2idx[an.Name.Name]; ok {
								vecE[idx] = an.Expr
							}
						}
					}
					tmp.Args = vecE

					// Enforce borrow (no move tracking here: IR for these is handled via exports)
					c.enforceCallsiteBorrow(chosen, &tmp)
					ret := chosen.Type.Ret
					if chosen.Decl != nil && chosen.Decl.Async {
						ret = types.FutureOf(ret)
					}
					c.info.Types[call] = ret
					return ret
				case 0:
					if !hasCands(set) {
						c.add(diagAt("DME0003", fe.Name.Span, base.Name+" has no exported '"+fe.Name.Name+"'"))
						return nil
					}
					c.add(diagAt("DTE0101", fe.Name.Span, "no matching overload"))
					return nil
				default:
					c.add(diagAt("DTE0102", fe.Name.Span, "ambiguous overload"))
					return nil
				}
			}
		}
		// Not a module import - try instance method call
		// Original instance method handling
		recvT := c.typ(fe.X)
		if recvT != nil {
			typeName := recvT.String()
			methodName := fe.Name.Name

			if impls, ok := c.info.Impls[typeName]; ok {
				var method *ast.FuncDecl
				for _, methods := range impls {
					for _, m := range methods {
						if m.Name.Name == methodName {
							method = m
							break
						}
					}
					if method != nil {
						break
					}
				}

				if method != nil {
					args := make([]types.T, len(argsNodes))
					for i, a := range argsNodes {
						args[i] = c.typ(a.Expr)
					}

					if len(args) != len(method.Params) {
						c.add(diagAt("DTE0046", fe.Name.Span, "arity mismatch"))
						return nil
					}

					for i := range args {
						var paramT types.T = types.None
						if method.Params[i].Type != nil {
							if t, ok := types.FromName(method.Params[i].Type.Name); ok {
								paramT = t
							}
						}
						if !types.Assignable(paramT, args[i]) {
							c.add(diagAt("DTE0104", argsNodes[i].Expr.SpanOf(), "argument type mismatch"))
						}
					}

					var retT types.T = types.None
					if method.RetType != nil {
						if t, ok := types.FromName(method.RetType.Name); ok {
							retT = t
						}
					}
					c.info.Types[call] = retT
					return retT
				}
			}
		}

		// Handle instance method calls for built-in collection types (set, dict, list)
		// ONLY if not handled by trait/custom type logic above
		// Check the receiver type - only handle our built-in collection types
		receiverType := c.typ(fe.X)
		if receiverType != nil {
			// Only handle built-in collection types, not custom structs/types
			isBuiltinCollection := false
			switch receiverType.(type) {
			case *types.Set, *types.Dict, *types.List:
				isBuiltinCollection = true
			}

			if isBuiltinCollection {
				// Get the method type from the field expression
				methodType := c.typFieldExpr(fe)
				if methodType != nil {
					// Type check arguments
					args := make([]types.T, len(argsNodes))
					for i, a := range argsNodes {
						args[i] = c.typ(a.Expr)
					}

					// If methodType is a function type, extract and store its return type
					if funcType, ok := methodType.(*types.Func); ok {
						c.info.Types[call] = funcType.Ret
						return funcType.Ret
					}
				}
			}
		}

		// Handle enum variant constructor calls: EnumType.Variant(...)
		// The FieldExpr (e.g., Status.Pending) should have been resolved to a function type by typFieldExpr
		// We hust need to extract and return the return type
		constructorType := c.typFieldExpr(fe)
		if constructorType != nil {
			if funcType, ok := constructorType.(*types.Func); ok {
				// Type check arguments against function parameters
				args := make([]types.T, len(argsNodes))
				for i, a := range argsNodes {
					args[i] = c.typ(a.Expr)
				}

				// Simple arity check
				if len(args) != len(funcType.Params) {
					c.add(diagAt("DTE0046", fe.Name.Span, "arity mismatch"))
					return nil
				}

				// Generic Type Inference
				// If the return type is a generic Enum, try to infer type parameters
				var inferredArgs []types.T
				var enumType *types.Enum
				if et, ok := funcType.Ret.(*types.Enum); ok && len(et.TypeParams) > 0 {
					enumType = et
					// Initialize inferred args with nil
					inferredArgs = make([]types.T, len(et.TypeParams))

					// Map param name -> index
					paramIdx := make(map[string]int)
					for i, tp := range et.TypeParams {
						paramIdx[tp.Name] = i
					}

					// Infer from arguments
					for i, paramT := range funcType.Params {
						if tp, ok := paramT.(*types.TypeParam); ok {
							if idx, found := paramIdx[tp.Name]; found {
								if inferredArgs[idx] == nil {
									inferredArgs[idx] = args[i]
								} else {
									// Check for conflict
									if !types.Equal(inferredArgs[idx], args[i]) {
										c.add(diagAt("DTE0104", argsNodes[i].Expr.SpanOf(), "conflicting type inference for "+tp.Name))
									}
								}
								continue // Skip standard assignable check for now
							}
						}
					}
				}

				// Type check
				for i := range args {
					// If param is TypeParam, we already handled it (or it's unconstrained)
					if _, ok := funcType.Params[i].(*types.TypeParam); ok {
						continue
					}
					if !types.Assignable(funcType.Params[i], args[i]) {
						c.add(diagAt("DTE0104", argsNodes[i].Expr.SpanOf(), "argument type mismatch"))
					}
				}

				// Construct return type
				retType := funcType.Ret
				if enumType != nil {
					// Verify all params inferred
					allInferred := true
					for _, t := range inferredArgs {
						if t == nil {
							allInferred = false
							break
						}
					}

					if allInferred {
						retType = &types.Generic{
							Base: enumType,
							Args: inferredArgs,
						}
					} else {
						// If not all inferred, maybe we can't instantiate yet?
						// For now, return Generic with Any or error?
						// Or maybe the user provided explicit type args?
						// But this path is for implicit constructor call.
						// If inference fails, we might return raw Enum (which is wrong) or error.
						// Let's assume for now simple cases work.
					}
				}

				c.info.Types[call] = retType
				return retType
			}
		}
		return nil
	}

	// --- Case 2: plain identifier call: f(...)
	if id, ok := call.Callee.(*ast.Ident); ok {
		set := c.info.Funcs[id.Name]
		sym := c.scope.Lookup(id.Name)
		isCallableSym := sym != nil && sym.Kind == SymFunc
		isTypeSym := sym != nil && sym.Kind == SymType
		callable := isCallableSym || isTypeSym || (set != nil && len(set.Cands) > 0)
		if !callable {
			if sym == nil {
				c.add(diagAt("DTE0001", id.Span, "undefined function: "+id.Name))
				return nil
			}
			c.add(diagAt("DTE0105", id.Span, "value is not callable"))
			return nil
		}

		if isTypeSym {
			return c.checkTypeCall(call, sym)
		}

		if !hasNamed {
			// Legacy positional path
			args := make([]types.T, len(argsNodes))
			for i, a := range argsNodes {
				args[i] = c.typ(a.Expr)
			}
			// M14 Stage 3: Special handling for print(Display) - check AFTER arg types computed
			if id.Name == "print" && len(args) == 1 && args[0] != nil {
				typeName := args[0].String()
				if impls, ok := c.info.Impls[typeName]; ok {
					if _, hasDisplay := impls["Display"]; hasDisplay {
						c.info.Types[call] = types.None
						return types.None
					}
				}
			}

			if set == nil || len(set.Cands) == 0 {
				c.add(diagAt("DTE0105", call.Callee.SpanOf(), "value is not callable"))
				return nil
			}

			// M15: Generic Function Inference
			// If candidates are generic, try to infer type arguments
			var inferredCands []*FuncCand
			for _, cand := range set.Cands {
				if len(cand.Type.TypeParams) > 0 {
					// Generic candidate
					inferred := make(map[string]types.T)
					// Unify args with params
					// Note: filterExactByTypes checks assignability, but for generics we need unification first.

					// Check arity first
					if !cand.Type.Variadic && len(args) != len(cand.Type.Params) {
						continue
					}
					// TODO: Handle variadic generics if needed

					match := true
					for i, argT := range args {
						if i >= len(cand.Type.Params) {
							break
						}
						paramT := cand.Type.Params[i]
						if !unify(paramT, argT, inferred) {
							match = false
							break
						}
					}

					if match {
						// Verify all type params are inferred
						allInferred := true
						for _, tp := range cand.Type.TypeParams {
							if _, ok := inferred[tp.Name]; !ok {
								allInferred = false
								break
							}
						}

						if allInferred {
							// Instantiate the candidate
							newType := substitute(cand.Type, inferred).(*types.Func)
							// Create a new candidate with instantiated type
							newCand := &FuncCand{
								Decl:       cand.Decl,
								Type:       newType,
								Modes:      cand.Modes,
								Extern:     cand.Extern,
								Defaults:   cand.Defaults,
								ParamNames: cand.ParamNames,
							}
							inferredCands = append(inferredCands, newCand)
						}
					}
				} else {
					// Non-generic candidate
					inferredCands = append(inferredCands, cand)
				}
			}

			// Use inferred candidates for filtering
			arityCands := filterByArity(inferredCands, len(args))
			if len(arityCands) == 0 {
				c.add(diagAt("DTE0046", id.Span, "arity mismatch: wrong number of arguments"))
				return nil
			}
			exact := filterExactByTypes(arityCands, args)
			switch len(exact) {
			case 1:
				chosen := exact[0]
				if chosen.Extern && c.unsafeDepth == 0 {
					c.add(diagAt("DFI0003", call.Callee.SpanOf(), ""))
				}
				// Synthesize positional Args for borrow/move enforcement
				tmp := *call
				tmp.Args = make([]ast.Expr, len(argsNodes))
				for i, a := range argsNodes {
					tmp.Args[i] = a.Expr
				}
				c.markMovesFromCall(chosen, &tmp, args)
				c.enforceCallsiteBorrow(chosen, &tmp)

				ret := chosen.Type.Ret
				if chosen.Decl != nil && chosen.Decl.Async {
					ret = types.FutureOf(ret)
				}
				c.info.Types[call] = ret
				return ret
			case 0:
				c.add(diagAt("DTE0101", id.Span, "no matching overload"))
				return nil
			default:
				c.add(diagAt("DTE0102", id.Span, "ambiguous overload"))
				return nil
			}
		}

		// Named-args path - only execute if there are named args
		if hasNamed {
			var exact []*FuncCand
			for _, cand := range set.Cands {
				vec, ok := c.canonicalizeForCandidate(cand, argsNodes)
				if !ok {
					continue
				}
				if typesMatchExactly(cand.Type.Params, vec, cand.Type.Variadic) {
					exact = append(exact, cand)
				}
			}
			switch len(exact) {
			case 1:
				chosen := exact[0]
				if chosen.Extern && c.unsafeDepth == 0 {
					c.add(diagAt("DFI0003", call.Callee.SpanOf(), ""))
				}
				// Build positional Args vector aligned to params for borrow/move
				tmp := *call
				vecE := make([]ast.Expr, len(chosen.Type.Params))
				// Map names -> indices
				pnames := c.paramNamesForCand(chosen)
				name2idx := map[string]int{}
				for i, nm := range pnames {
					if nm != "" {
						name2idx[nm] = i
					}
				}
				// Fill leading positionals
				pos := 0
				for _, an := range argsNodes {
					if an.Name == nil {
						if pos < len(vecE) {
							vecE[pos] = an.Expr
							pos++
						}
					}
				}
				// Fill named
				for _, an := range argsNodes {
					if an.Name != nil {
						if idx, ok := name2idx[an.Name.Name]; ok {
							vecE[idx] = an.Expr
						}
					}
				}
				tmp.Args = vecE

				// Types for move tracking (aligned)
				vecT := make([]types.T, len(vecE))
				for i := range vecE {
					vecT[i] = c.typ(vecE[i])
				}

				c.markMovesFromCall(chosen, &tmp, vecT)
				c.enforceCallsiteBorrow(chosen, &tmp)

				ret := chosen.Type.Ret
				if chosen.Decl != nil && chosen.Decl.Async {
					ret = types.FutureOf(ret)
				}
				c.info.Types[call] = ret
				return ret
			case 0:
				c.add(diagAt("DTE0101", id.Span, "no matching overload"))
				return nil
			default:
				c.add(diagAt("DTE0102", id.Span, "ambiguous overload"))
				return nil
			}
		}
	}

	// --- Fallback: callee is some other expression (e.g., (1)()).
	_ = c.typ(call.Callee)
	for _, a := range argsNodes {
		_ = c.typ(a.Expr)
	}
	c.add(diagAt("DTE0105", call.Callee.SpanOf(), "value is not callable"))
	return nil
}

/* ----------------------------- named args core ---------------------------- */

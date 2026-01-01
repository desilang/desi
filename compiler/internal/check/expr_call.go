package check

import (
	"fmt"

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

	// --- Case 1: module-qualified call: mod.fn(...) or Class.method() or Outer.Inner()
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
			// Also check for str type (for split, replace methods)
			if types.Equal(receiverType, types.Str) {
				isBuiltinCollection = true
			}
			// Also check for File type (for read, write, close methods)
			if types.Equal(receiverType, types.File) {
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

					// If methodType is a function type, validate arguments and extract return type
					if funcType, ok := methodType.(*types.Func); ok {
						// Check arity
						if len(args) != len(funcType.Params) {
							c.add(diagAt("DTE0046", fe.Name.Span, "arity mismatch"))
							return nil
						}

						// Check argument types
						for i := range args {
							if args[i] != nil && funcType.Params[i] != nil {
								// DEBUG: Print types being compared
								// fmt.Printf("Checking arg %d: expected %s, got %s\n", i, funcType.Params[i].String(), args[i].String())

								if !types.Assignable(funcType.Params[i], args[i]) {
									c.add(diagAt("DTE0104", argsNodes[i].Expr.SpanOf(),
										fmt.Sprintf("argument type mismatch: expected %s, got %s",
											funcType.Params[i].String(), args[i].String())))
								}
							}
						}

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

			// Handle nested class constructor calls: Outer.Inner()
			// typFieldExpr returns *types.Class for nested class access
			if classType, ok := constructorType.(*types.Class); ok {
				// Create a synthetic symbol for the nested class and delegate to checkTypeCall
				nestedSym := &Symbol{
					Name: fe.Name.Name,
					Kind: SymType,
					Type: classType,
					Node: classType.Decl,
				}
				return c.checkTypeCall(call, nestedSym)
			}
		}
		return nil

	}

	// --- Case 2: plain identifier call: f(...)
	if id, ok := call.Callee.(*ast.Ident); ok {
		// Special case: print() is variadic and accepts any number of args of any type
		// Also supports keyword args: sep (separator), end (terminator), file (stream), flush (bool)
		if id.Name == "print" {
			// Type-check all arguments and validate kwargs
			if len(call.ArgNodes) > 0 {
				for _, a := range call.ArgNodes {
					// Check for known kwargs
					if a.Name != nil {
						kwName := a.Name.Name
						if kwName == "sep" || kwName == "end" || kwName == "file" || kwName == "flush" {
							// file=sys.stdout/stderr are magic - don't type-check them
							if kwName == "file" {
								// Check if it's sys.stdout or sys.stderr
								if fe, ok := a.Expr.(*ast.FieldExpr); ok {
									if id, ok := fe.X.(*ast.Ident); ok && id.Name == "sys" {
										if fe.Name.Name == "stdout" || fe.Name.Name == "stderr" {
											continue // Skip type-checking for magic sys streams
										}
									}
								}
							}
							// sep and end must be strings
							argType := c.typ(a.Expr)
							if kwName == "sep" || kwName == "end" {
								if argType != nil && !types.Equal(argType, types.Str) {
									c.add(diagAt("DTE0001", a.Expr.SpanOf(), fmt.Sprintf("print() %s must be a string", kwName)))
								}
							}
							// flush should be bool (validated elsewhere)
						} else {
							c.add(diagAt("DTE0001", a.Name.Span, fmt.Sprintf("print() got unexpected keyword argument '%s'", kwName)))
						}
					} else {
						// Positional arg - type-check it
						c.typ(a.Expr)
					}
				}
			} else {
				for _, a := range call.Args {
					c.typ(a)
				}
			}
			c.info.Types[call] = types.None
			return types.None
		}
		set := c.info.Funcs[id.Name]
		sym := c.scope.Lookup(id.Name)
		isCallableSym := sym != nil && sym.Kind == SymFunc
		isTypeSym := sym != nil && sym.Kind == SymType
		// Check if variable has a Func type (e.g., lambda stored in a variable)
		isFuncTypedVar := sym != nil && sym.Kind == SymVar && sym.Type != nil
		var funcType *types.Func
		if isFuncTypedVar {
			funcType, _ = sym.Type.(*types.Func)
			isFuncTypedVar = funcType != nil
		}
		callable := isCallableSym || isTypeSym || isFuncTypedVar || (set != nil && len(set.Cands) > 0)
		if !callable {
			if sym == nil {
				c.add(diagAt("DTE0001", id.Span, "undefined function: "+id.Name))
				return nil
			}
			c.add(diagAt("DTE0105", id.Span, "value is not callable"))
			return nil
		}

		// Handle calling a Func-typed variable (lambda stored in variable)
		if isFuncTypedVar && funcType != nil {
			args := make([]types.T, len(argsNodes))
			for i, a := range argsNodes {
				args[i] = c.typ(a.Expr)
			}
			// Check arity
			if len(args) != len(funcType.Params) && !funcType.Variadic {
				c.add(diagAt("DTE0046", call.Span, "arity mismatch: wrong number of arguments"))
				return nil
			}
			// Check arg types
			for i := range funcType.Params {
				if i >= len(args) {
					break
				}
				if !types.Equal(args[i], funcType.Params[i]) {
					c.add(diagAt("DTE0104", call.Span, fmt.Sprintf("argument type mismatch at position %d", i+1)))
					return nil
				}
			}
			c.info.Types[call] = funcType.Ret
			return funcType.Ret
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
			// M14 Stage 3: Special handling for print(Display) + Auto to_str
			// Accept: Display trait, collections (list/dict/set), custom classes with __str__/to_str
			// Built-in len() function
			if id.Name == "len" && len(args) == 1 && args[0] != nil {
				argType := args[0]
				if argType == nil {
					return nil
				}

				// Unwrap TypeAlias to check underlying type
				if ta, ok := argType.(*types.TypeAlias); ok {
					argType = ta.Target
				}

				// Special case for string
				if types.Equal(argType, types.Str) {
					c.info.Types[call] = types.Int
					return types.Int
				}

				// Check for __len__ method
				hasLen := false
				if _, ok := argType.(*types.List); ok {
					hasLen = true
				} else if _, ok := argType.(*types.Dict); ok {
					hasLen = true
				} else if _, ok := argType.(*types.Set); ok {
					hasLen = true
				} else if _, ok := argType.(*types.Tuple); ok {
					hasLen = true
				} else if cls, ok := argType.(*types.Class); ok {
					// Check for __len__ method in class or base classes
					curr := cls
					for curr != nil {
						if _, ok := curr.Dunders["__len__"]; ok {
							hasLen = true
							break
						}
						curr = curr.Base
					}
				}

				if hasLen {
					c.info.Types[call] = types.Int
					return types.Int
				}

				c.add(diagAt("DTE0001", call.Span, "type '"+argType.String()+"' has no len()"))
				return nil
			}

			// Built-in print() function - variadic, accepts any number of arguments of any type
			if id.Name == "print" {
				// print() accepts any number of arguments - all are valid
				// Return type is none
				c.info.Types[call] = types.None
				return types.None
			}

			// Built-in enumerate() function - accepts any iterable
			if id.Name == "enumerate" && len(args) == 1 && args[0] != nil {
				argType := args[0]
				isIterable := false
				switch argType.(type) {
				case *types.List, *types.Set:
					isIterable = true
				}
				if isIterable {
					// enumerate returns an iterator - for type checking in for-loops,
					// we mark it as returning the inner type (handled specially in stmt.go)
					c.info.Types[call] = argType
					return argType
				}
				c.add(diagAt("DTE0001", call.Span, "enumerate requires an iterable (list or set)"))
				return nil
			}

			// Built-in reversed() function - accepts any iterable
			if id.Name == "reversed" && len(args) == 1 && args[0] != nil {
				argType := args[0]
				isIterable := false
				switch argType.(type) {
				case *types.List, *types.Set:
					isIterable = true
				}
				if isIterable {
					c.info.Types[call] = argType
					return argType
				}
				c.add(diagAt("DTE0001", call.Span, "reversed requires an iterable (list or set)"))
				return nil
			}

			// Built-in zip() function - accepts two iterables
			if id.Name == "zip" && len(args) == 2 && args[0] != nil && args[1] != nil {
				isIterable1, isIterable2 := false, false
				switch args[0].(type) {
				case *types.List, *types.Set:
					isIterable1 = true
				}
				switch args[1].(type) {
				case *types.List, *types.Set:
					isIterable2 = true
				}
				if isIterable1 && isIterable2 {
					// Return a tuple of both types (for type checking in for-loop stmt.go)
					c.info.Types[call] = types.TupleOf(args[0], args[1])
					return c.info.Types[call]
				}
				c.add(diagAt("DTE0001", call.Span, "zip requires two iterables (list or set)"))
				return nil
			}

			// Built-in sum() function - accepts list[int], list[float], or homogeneous tuple
			if id.Name == "sum" && len(args) == 1 && args[0] != nil {
				if listT, ok := args[0].(*types.List); ok {
					if types.Equal(listT.Elem, types.Int) || types.Equal(listT.Elem, types.Float) {
						c.info.Types[call] = listT.Elem
						return listT.Elem
					}
				}
				// Also accept homogeneous tuple of int or float
				if tupT, ok := args[0].(*types.Tuple); ok && len(tupT.Elems) > 0 {
					firstType := tupT.Elems[0]
					if types.Equal(firstType, types.Int) || types.Equal(firstType, types.Float) {
						allSame := true
						for _, e := range tupT.Elems {
							if !types.Equal(e, firstType) {
								allSame = false
								break
							}
						}
						if allSame {
							c.info.Types[call] = firstType
							return firstType
						}
					}
				}
				c.add(diagAt("DTE0001", call.Span, "sum requires list[int], list[float], or homogeneous tuple"))
				return nil
			}

			// Built-in min()/max() functions - accepts list[int], list[float], or homogeneous tuple
			if (id.Name == "min" || id.Name == "max") && len(args) == 1 && args[0] != nil {
				if listT, ok := args[0].(*types.List); ok {
					if types.Equal(listT.Elem, types.Int) || types.Equal(listT.Elem, types.Float) {
						c.info.Types[call] = listT.Elem
						return listT.Elem
					}
				}
				// Also accept homogeneous tuple of int or float
				if tupT, ok := args[0].(*types.Tuple); ok && len(tupT.Elems) > 0 {
					firstType := tupT.Elems[0]
					if types.Equal(firstType, types.Int) || types.Equal(firstType, types.Float) {
						allSame := true
						for _, e := range tupT.Elems {
							if !types.Equal(e, firstType) {
								allSame = false
								break
							}
						}
						if allSame {
							c.info.Types[call] = firstType
							return firstType
						}
					}
				}
				c.add(diagAt("DTE0001", call.Span, id.Name+" requires list[int], list[float], or homogeneous tuple"))
				return nil
			}
			// Built-in any()/all() functions - accepts list[bool]
			if (id.Name == "any" || id.Name == "all") && len(args) == 1 && args[0] != nil {
				if listT, ok := args[0].(*types.List); ok {
					if types.Equal(listT.Elem, types.Bool) {
						c.info.Types[call] = types.Bool
						return types.Bool
					}
				}
				c.add(diagAt("DTE0001", call.Span, id.Name+" requires list[bool]"))
				return nil
			}

			// Built-in sorted() function - accepts list[int], returns list[int]
			if id.Name == "sorted" && len(args) == 1 && args[0] != nil {
				if listT, ok := args[0].(*types.List); ok {
					if types.Equal(listT.Elem, types.Int) {
						c.info.Types[call] = args[0]
						return args[0]
					}
				}
				c.add(diagAt("DTE0001", call.Span, "sorted requires list[int]"))
				return nil
			}

			// Built-in mutex_new(value) function - creates Mutex[T] from value type
			if id.Name == "mutex_new" && len(args) == 1 && args[0] != nil {
				mutexType := types.MutexOf(args[0])
				c.info.Types[call] = mutexType
				return mutexType
			}

			// Built-in channel_new(capacity) function - creates Channel[Any]
			// For typed channels, use: let ch = channel_new_int(10) etc.
			// or wait for generic function syntax: channel_new[int](10)
			if id.Name == "channel_new" && len(args) == 1 {
				// Without generic syntax, channel_new creates Channel[Any]
				channelType := types.ChannelOf(types.Any)
				c.info.Types[call] = channelType
				return channelType
			}

			// Built-in rc(value) function - creates Rc[T] from value type
			if id.Name == "rc" && len(args) == 1 && args[0] != nil {
				rcType := types.RcOf(args[0])
				c.info.Types[call] = rcType
				return rcType
			}

			// Built-in arc(value) function - creates Arc[T] from value type
			if id.Name == "arc" && len(args) == 1 && args[0] != nil {
				arcType := types.ArcOf(args[0])
				c.info.Types[call] = arcType
				return arcType
			}

			// Built-in reduce(), foldl(), foldr() functions
			// Signature: reduce(func, iterable, initial) -> AccumulatorType
			// foldl is alias for reduce (left-to-right)
			// foldr processes right-to-left
			if (id.Name == "reduce" || id.Name == "foldl" || id.Name == "foldr") && len(args) == 3 {
				funcArg := args[0]
				iterArg := args[1]
				initArg := args[2]

				if funcArg == nil || iterArg == nil || initArg == nil {
					c.add(diagAt("DTE0001", call.Span, id.Name+" requires (func, iterable, initial)"))
					return nil
				}

				// Check that second arg is iterable (list or set)
				var elemType types.T
				switch it := iterArg.(type) {
				case *types.List:
					elemType = it.Elem
				case *types.Set:
					elemType = it.Elem
				default:
					c.add(diagAt("DTE0001", call.Span, id.Name+" requires iterable as second argument"))
					return nil
				}

				// Check that first arg is a function: (AccT, ElemT) -> AccT
				funcType, ok := funcArg.(*types.Func)
				if !ok {
					c.add(diagAt("DTE0001", call.Span, id.Name+" requires function as first argument"))
					return nil
				}

				// Function must take exactly 2 parameters
				if len(funcType.Params) != 2 {
					c.add(diagAt("DTE0001", call.Span, id.Name+" function must take exactly 2 parameters (acc, elem)"))
					return nil
				}

				// The accumulator type is determined by the initial value
				accType := initArg

				// Verify function signature: (AccT, ElemT) -> AccT
				// param[0] should be assignable from accType
				// param[1] should be assignable from elemType
				// return should be assignable to accType
				if !types.Assignable(funcType.Params[0], accType) {
					c.add(diagAt("DTE0104", call.Span, id.Name+" function first param must match initial value type"))
					return nil
				}
				if !types.Assignable(funcType.Params[1], elemType) {
					c.add(diagAt("DTE0104", call.Span, id.Name+" function second param must match element type"))
					return nil
				}
				if !types.Assignable(accType, funcType.Ret) {
					c.add(diagAt("DTE0104", call.Span, id.Name+" function return type must match accumulator type"))
					return nil
				}

				// Result type is the accumulator type
				c.info.Types[call] = accType
				return accType
			}

			if id.Name == "print" && len(args) == 1 && args[0] != nil {
				shouldAccept := false

				// Case 1: Display trait
				typeName := args[0].String()
				if impls, ok := c.info.Impls[typeName]; ok {
					if _, hasDisplay := impls["Display"]; hasDisplay {
						shouldAccept = true
					}
				}

				// Case 2: Built-in collections (list, dict, set)
				if !shouldAccept {
					switch args[0].(type) {
					case *types.List, *types.Dict, *types.Set:
						shouldAccept = true
					case *types.Class:
						// Case 3: Custom class with __str__ or to_str dunder
						cls := args[0].(*types.Class)
						if _, found := cls.Dunders["__str__"]; found {
							shouldAccept = true
						} else if _, found := cls.Dunders["to_str"]; found {
							shouldAccept = true
						}
					}
				}

				if shouldAccept {
					c.info.Types[call] = types.None
					return types.None
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

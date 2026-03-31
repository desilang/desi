package check

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/macro"
	"github.com/desilang/desi/compiler/internal/types"
)

// isFCall delegates to macro.IsFCall for backward compatibility.
// Checks if an expression is an F() call or contains one in a binary expression.
func isFCall(expr ast.Expr) bool {
	return macro.IsFCall(expr)
}

func (c *checker) typBinary(x *ast.BinaryExpr) types.T {
	op := x.Op

	// Helpers: classify numeric families via canonical String() spellings.
	intInfo := func(t types.T) (signed bool, width int, ok bool) {
		if t == nil {
			return false, 0, false
		}
		switch t.String() {
		case "int":
			return true, 0, true // width==0 marks unsized 'int'
		case "isize":
			return true, 64, true
		case "usize":
			return false, 64, true
		case "i8":
			return true, 8, true
		case "i16":
			return true, 16, true
		case "i32":
			return true, 32, true
		case "i64":
			return true, 64, true
		case "i128":
			return true, 128, true
		case "u8":
			return false, 8, true
		case "u16":
			return false, 16, true
		case "u32":
			return false, 32, true
		case "u64":
			return false, 64, true
		case "u128":
			return false, 128, true
		default:
			return false, 0, false
		}
	}
	floatInfo := func(t types.T) (width int, ok bool) {
		if t == nil {
			return 0, false
		}
		switch t.String() {
		case "f32":
			return 32, true
		case "float", "f64": // 'float' is our f64 alias
			return 64, true
		default:
			return 0, false
		}
	}

	switch op {
	case "+", "-", "*", "/", "%", "**", "|", "&", "^", "<<", ">>":
		lt := c.typ(x.Lhs)
		rt := c.typ(x.Rhs)

		// Decimal arithmetic: decimal + decimal, decimal - decimal, etc.
		if types.Equal(lt, types.Decimal) && types.Equal(rt, types.Decimal) {
			if op == "+" || op == "-" || op == "*" || op == "/" {
				c.info.Types[x] = types.Decimal
				return types.Decimal
			}
		}

		// Operator overloading for classes
		if lt != nil {
			if cls, ok := lt.(*types.Class); ok {
				// Map operator to dunder method name
				var dunder string
				switch op {
				case "+":
					dunder = "__add__"
				case "-":
					dunder = "__sub__"
				case "*":
					dunder = "__mul__"
				case "/":
					dunder = "__div__"
				case "%":
					dunder = "__mod__"
				case "**":
					dunder = "__pow__"
				case "|":
					dunder = "__or__"
				case "&":
					dunder = "__and__"
				case "^":
					dunder = "__xor__"
				case "<<":
					dunder = "__lshift__"
				case ">>":
					dunder = "__rshift__"
				}

				if dunder != "" {
					// Look up method in class's Dunders map
					if ft, ok := cls.Dunders[dunder]; ok {
						// Found overload!
						// Check if it accepts the right operand type
						if len(ft.Params) == 2 { // self + other
							// Check if rt matches the second parameter
							if types.Equal(rt, ft.Params[1]) {
								c.info.Types[x] = ft.Ret
								// Create a FuncCand for the overload
								cand := &FuncCand{
									Type: ft,
								}
								c.info.BinOpOverloads[x] = cand
								return ft.Ret
							}
						}
					}
				}
			}
		}

		// String ergonomics: allow str + (int|float|bool|str|Display) => str
		if op == "+" && (types.Equal(lt, types.Str) || types.Equal(rt, types.Str)) {
			if types.Equal(lt, types.Str) && types.Equal(rt, types.Str) {
				c.info.Types[x] = types.Str
				return types.Str
			}
			other := rt
			if types.Equal(lt, types.Str) {
				other = rt
			} else {
				other = lt
			}
			// Check if other type is allowed
			allowed := types.Equal(other, types.Int) || types.Equal(other, types.Float) ||
				types.Equal(other, types.Bool) || types.Equal(other, types.Str)

			// Also allow if type implements Display trait
			if !allowed {
				if st, ok := other.(*types.Struct); ok {
					typeName := st.Name
					if impls, ok := c.info.Impls[typeName]; ok {
						if _, hasDisplay := impls["Display"]; hasDisplay {
							allowed = true
						}
					}
				}
			}

			if allowed {
				c.info.Types[x] = types.Str
				return types.Str
			}
		}

		// Tuple concatenation: tuple + tuple => combined tuple
		if op == "+" {
			ltup, lok := lt.(*types.Tuple)
			rtup, rok := rt.(*types.Tuple)
			if lok && rok {
				// Create new tuple with combined elements
				combined := make([]types.T, 0, len(ltup.Elems)+len(rtup.Elems))
				combined = append(combined, ltup.Elems...)
				combined = append(combined, rtup.Elems...)
				result := types.TupleOf(combined...)
				c.info.Types[x] = result
				return result
			}
		}

		// Integers: same signedness & same width, with one exception:
		//   M9 allowance: (isize|usize) with unsized int ==> OK, result keeps pointer-sized type.
		if ls, lw, lok := intInfo(lt); lok {
			if rs, rw, rok := intInfo(rt); rok {
				allowIntPtrMix := (lw == 0 && (rw == 64)) || (rw == 0 && (lw == 64))
				if !allowIntPtrMix {
					if ls != rs {
						c.add(diagAt("DNT0002", x.Span, "")) // signed/unsigned mismatch
						return nil
					}
					if lw != rw {
						c.add(diagAt("DNT0001", x.Span, "")) // width mismatch
						return nil
					}
					// OK: same family — result type is the left (equal to right)
					c.info.Types[x] = lt
					return lt
				}
				// allow int <op> (isize|usize)
				if lw == 0 && rw == 64 {
					c.info.Types[x] = rt
					return rt
				}
				if rw == 0 && lw == 64 {
					c.info.Types[x] = lt
					return lt
				}
			}
		}

		// Floats: require same width (f32 with f32; f64/float with f64/float)
		// Bitwise operators are NOT allowed on floats
		if op != "|" && op != "&" && op != "^" && op != "<<" && op != ">>" {
			if lw, lok := floatInfo(lt); lok {
				if rw, rok := floatInfo(rt); rok {
					if lw != rw {
						c.add(diagAt("DNT0001", x.Span, ""))
						return nil
					}
					if lw == 32 {
						c.info.Types[x] = types.F32
						return types.F32
					}
					c.info.Types[x] = types.Float // f64 alias
					return types.Float
				}
			}
		}

		// Q object operators: Q(...) | Q(...) and Q(...) & Q(...)
		// Q returns str, so str | str and str & str are valid Q combinations
		if (op == "|" || op == "&") && types.Equal(lt, types.Str) && types.Equal(rt, types.Str) {
			c.info.Types[x] = types.Str
			return types.Str
		}

		// F expression arithmetic: F("price") * 1.1, F("qty") + F("price")
		// Only when at least one side is an F() call
		if op == "-" || op == "*" || op == "/" {
			if isFCall(x.Lhs) || isFCall(x.Rhs) {
				isLStr := types.Equal(lt, types.Str)
				isRStr := types.Equal(rt, types.Str)
				isLNum := types.Equal(lt, types.Int) || types.Equal(lt, types.Float)
				isRNum := types.Equal(rt, types.Int) || types.Equal(rt, types.Float)
				if (isLStr && (isRNum || isRStr)) || (isRStr && isLNum) {
					c.info.Types[x] = types.Str
					return types.Str
				}
			}
		}

		// Any other combination is invalid for now.
		c.add(diagAt("DTE0004", x.Span, "invalid operands for '"+op+"'"))
		return nil

	case "|>":
		// pipeline: lhs |> f(a,b)  ==>  f(lhs, a, b)
		call, ok := x.Rhs.(*ast.CallExpr)
		if !ok {
			c.add(diagAt("DTE0103", x.Span, "pipeline expects a call on the right-hand side"))
			return nil
		}
		// Check for Identifier (function) or FieldExpr (method)
		var id *ast.Ident
		var field *ast.FieldExpr

		if ident, ok := call.Callee.(*ast.Ident); ok {
			id = ident
		} else if fe, ok := call.Callee.(*ast.FieldExpr); ok {
			field = fe
		} else {
			c.add(diagAt("DTE0103", x.Span, "pipeline target must be an identifier or method call"))
			return nil
		}

		// Case 1: Function call (Identifier)
		if id != nil {
			// Ensure LHS is typed
			_ = c.typ(x.Lhs)

			// Get RHS arguments (ArgNodes handles names)
			rhsArgs := call.ArgNodes
			if len(rhsArgs) == 0 && len(call.Args) > 0 {
				// Fallback if ArgNodes empty but Args present (should rely on parser populating ArgNodes)
				rhsArgs = make([]ast.CallArg, len(call.Args))
				for i, a := range call.Args {
					rhsArgs[i] = ast.CallArg{Expr: a}
				}
			}

			// Synthesize argument list: LHS + RHS args
			synthArgs := make([]ast.CallArg, 0, 1+len(rhsArgs))
			synthArgs = append(synthArgs, ast.CallArg{Name: nil, Expr: x.Lhs})
			synthArgs = append(synthArgs, rhsArgs...)

			// Create synthetic call expression sharing the same Callee (Identifier) and updated Args
			synthCall := &ast.CallExpr{
				Callee:   call.Callee,
				ArgNodes: synthArgs,
				Span:     x.Span,
			}

			// Delegate to typCall - this handles special builtins (sorted, print), overloading, named args, etc.
			ret := c.typCall(synthCall)
			if ret != nil {
				c.info.Types[x] = ret
				return ret
			}
			return nil

		} else if field != nil {
			// Case 2: Method call (FieldExpr)
			lhsT := c.typ(x.Lhs)
			recvT := c.typ(field.X)
			if recvT == nil {
				return nil
			}

			methodName := field.Name.Name

			// Resolve method on receiver type
			var method *types.Func
			if cls, ok := recvT.(*types.Class); ok {
				if m, ok := cls.Methods[methodName]; ok {
					method = m
				} else if m, ok := cls.StaticMethods[methodName]; ok {
					method = m
				}
			}

			if method == nil {
				c.add(diagAt("DTE0001", field.Name.Span, "undefined method: "+methodName))
				return nil
			}

			// Method Argument Matching
			// Assume instance method (skip first 'self' param) for now.
			paramOffset := 1
			if len(method.Params) < paramOffset {
				paramOffset = 0
			}

			effectiveParams := method.Params[paramOffset:]

			// Collect args: [lhs] + call.Args
			args := make([]types.T, 0, 1+len(call.Args))
			args = append(args, lhsT)
			for _, a := range call.Args {
				args = append(args, c.typ(a))
			}

			if len(effectiveParams) != len(args) {
				c.add(diagAt("DTE0046", x.Span, "pipeline method argument count mismatch"))
				return nil
			}

			for i, param := range effectiveParams {
				arg := args[i]
				if !types.Assignable(param, arg) {
					c.add(diagAt("DTE0004", x.Span, "pipeline method argument type mismatch"))
					return nil
				}
			}

			c.info.Types[x] = method.Ret
			return method.Ret
		}

		return nil

	case "<", "<=", ">", ">=",
		"==", "!=":
		lt := c.typ(x.Lhs)
		rt := c.typ(x.Rhs)

		// Operator overloading for classes (comparisons)
		if lt != nil {
			if cls, ok := lt.(*types.Class); ok {
				var dunder string
				switch op {
				case "==":
					dunder = "__eq__"
				case "!=":
					dunder = "__ne__"
				case "<":
					dunder = "__lt__"
				case "<=":
					dunder = "__le__"
				case ">":
					dunder = "__gt__"
				case ">=":
					dunder = "__ge__"
				}

				if dunder != "" {
					// Look up method in class's Dunders map
					if ft, ok := cls.Dunders[dunder]; ok {
						if len(ft.Params) == 2 {
							if types.Equal(rt, ft.Params[1]) {
								c.info.Types[x] = ft.Ret
								cand := &FuncCand{
									Type: ft,
								}
								c.info.BinOpOverloads[x] = cand
								return ft.Ret
							}
						}
					}

					// Fallback for != using __eq__
					if op == "!=" {
						if ft, ok := cls.Dunders["__eq__"]; ok {
							if len(ft.Params) == 2 {
								if types.Equal(rt, ft.Params[1]) {
									c.info.Types[x] = ft.Ret // Should be bool
									cand := &FuncCand{
										Type: ft,
									}
									c.info.BinOpOverloads[x] = cand // We'll need to negate this in lowering
									return ft.Ret
								}
							}
						}
					}
				}
			}
		}

		// Integers: same signedness & width, with the same M9 exception as above.
		if ls, lw, lok := intInfo(lt); lok {
			if rs, rw, rok := intInfo(rt); rok {
				allowIntPtrMix := (lw == 0 && (rw == 64)) || (rw == 0 && (lw == 64))
				if !allowIntPtrMix {
					if ls != rs {
						c.add(diagAt("DNT0002", x.Span, ""))
						return nil
					}
					if lw != rw {
						c.add(diagAt("DNT0001", x.Span, ""))
						return nil
					}
					c.info.Types[x] = types.Bool
					return types.Bool
				}
				c.info.Types[x] = types.Bool
				return types.Bool
			}
		}

		// Floats: same width
		if lw, lok := floatInfo(lt); lok {
			if rw, rok := floatInfo(rt); rok {
				if lw != rw {
					c.add(diagAt("DNT0001", x.Span, ""))
					return nil
				}
				c.info.Types[x] = types.Bool
				return types.Bool
			}
		}

		// Tuple comparison (lexicographic order requires homogeneous tuples)
		if ltup, lok := lt.(*types.Tuple); lok {
			if rtup, rok := rt.(*types.Tuple); rok {
				// For <, >, <=, >= require homogeneous tuples (all elements same type)
				if op == "<" || op == ">" || op == "<=" || op == ">=" {
					// Check if both tuples are homogeneous and comparable
					if len(ltup.Elems) == 0 || len(rtup.Elems) == 0 {
						c.info.Types[x] = types.Bool
						return types.Bool
					}

					// Verify homogeneous (all same type)
					firstType := ltup.Elems[0]
					allSame := true
					for _, e := range ltup.Elems {
						if !types.Equal(e, firstType) {
							allSame = false
							break
						}
					}
					for _, e := range rtup.Elems {
						if !types.Equal(e, firstType) {
							allSame = false
							break
						}
					}

					if !allSame {
						c.add(diagAt("DTE0004", x.Span, "tuple comparison requires homogeneous tuples"))
						return nil
					}

					c.info.Types[x] = types.Bool
					return types.Bool
				}

				// For == and != - already handled by tuple equality
				if types.Equal(lt, rt) {
					c.info.Types[x] = types.Bool
					return types.Bool
				}
			}
		}

		// Fallback: identical non-numeric types comparable
		if types.Equal(lt, rt) {
			c.info.Types[x] = types.Bool
			return types.Bool
		}
		c.add(diagAt("DTE0004", x.Span, "incomparable operands for '"+op+"'"))
		return nil

	case "and", "or":
		lt := c.typ(x.Lhs)
		rt := c.typ(x.Rhs)
		if types.Equal(lt, types.Bool) && types.Equal(rt, types.Bool) {
			c.info.Types[x] = types.Bool
			return types.Bool
		}
		c.add(diagAt("DTE0004", x.Span, "logical operators require bool operands"))
		return nil

	case "in":
		// Membership operator: x in collection
		lt := c.typ(x.Lhs)
		rt := c.typ(x.Rhs)

		// str in str -> bool (substring check)
		if lt != nil && rt != nil && types.Equal(lt, types.Str) && types.Equal(rt, types.Str) {
			c.info.Types[x] = types.Bool
			return types.Bool
		}

		// x in list[T] -> bool (element T matches x)
		if lt != nil && rt != nil {
			if listT, ok := rt.(*types.List); ok {
				if types.Assignable(listT.Elem, lt) {
					c.info.Types[x] = types.Bool
					return types.Bool
				}
				c.add(diagAt("DTE0004", x.Span, "element type '"+lt.String()+"' doesn't match list element type '"+listT.Elem.String()+"'"))
				return nil
			}
		}

		// x in set[T] -> bool (element T matches x)
		if lt != nil && rt != nil {
			if setT, ok := rt.(*types.Set); ok {
				if types.Assignable(setT.Elem, lt) {
					c.info.Types[x] = types.Bool
					return types.Bool
				}
				c.add(diagAt("DTE0004", x.Span, "element type '"+lt.String()+"' doesn't match set element type '"+setT.Elem.String()+"'"))
				return nil
			}
		}

		// x in dict[K,V] -> bool (key K matches x)
		if lt != nil && rt != nil {
			if dictT, ok := rt.(*types.Dict); ok {
				if types.Assignable(dictT.Key, lt) {
					c.info.Types[x] = types.Bool
					return types.Bool
				}
				c.add(diagAt("DTE0004", x.Span, "key type '"+lt.String()+"' doesn't match dict key type '"+dictT.Key.String()+"'"))
				return nil
			}
		}

		// x in tuple (homogeneous tuple only)
		if lt != nil && rt != nil {
			if tupT, ok := rt.(*types.Tuple); ok {
				if len(tupT.Elems) > 0 {
					// Check if tuple is homogeneous and element type matches LHS
					firstType := tupT.Elems[0]
					allSame := true
					for _, e := range tupT.Elems {
						if !types.Equal(e, firstType) {
							allSame = false
							break
						}
					}
					if allSame && types.Equal(lt, firstType) {
						c.info.Types[x] = types.Bool
						return types.Bool
					}
					if !allSame {
						c.add(diagAt("DTE0004", x.Span, "'in' requires homogeneous tuple"))
						return nil
					}
					c.add(diagAt("DTE0004", x.Span, "element type '"+lt.String()+"' doesn't match tuple element type '"+firstType.String()+"'"))
					return nil
				}
				// Empty tuple: always returns false but is valid
				c.info.Types[x] = types.Bool
				return types.Bool
			}
		}

		// Custom class with __contains__ dunder
		if lt != nil && rt != nil {
			if cls, ok := rt.(*types.Class); ok {
				if ft, found := cls.Dunders["__contains__"]; found {
					if len(ft.Params) == 2 && types.Assignable(ft.Params[1], lt) {
						c.info.Types[x] = types.Bool
						return types.Bool
					}
				}
			}
		}

		c.add(diagAt("DTE0004", x.Span, "unsupported 'in' operands"))
		return nil

	default:
		return nil
	}
}

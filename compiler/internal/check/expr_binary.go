package check

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/types"
)

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
		id, ok := call.Callee.(*ast.Ident)
		if !ok || id == nil {
			c.add(diagAt("DTE0103", x.Span, "pipeline target must be an identifier"))
			return nil
		}

		lhsT := c.typ(x.Lhs)
		set := c.info.Funcs[id.Name]
		if set == nil || len(set.Cands) == 0 {
			c.add(diagAt("DTE0001", id.Span, "pipeline target undefined function: "+id.Name))
			return nil
		}

		base := callArgs(call)
		hasNamed := false
		for _, a := range base {
			if a.Name != nil {
				hasNamed = true
				break
			}
		}

		if !hasNamed {
			// Legacy positional: prepend lhs, then resolve
			args := make([]types.T, 0, 1+len(base))
			args = append(args, lhsT)
			for _, a := range base {
				args = append(args, c.typ(a.Expr))
			}
			arityCands := filterByArity(set.Cands, len(args))
			if len(arityCands) == 0 {
				c.add(diagAt("DTE0046", x.Span, "pipeline arity mismatch"))
				return nil
			}
			exact := filterExactByTypes(arityCands, args)
			switch len(exact) {
			case 1:
				chosen := exact[0]
				if chosen.Extern && c.unsafeDepth == 0 {
					c.add(diagAt("DFI0003", call.Callee.SpanOf(), ""))
				}
				ret := chosen.Type.Ret
				c.info.Types[call.Callee] = chosen.Type
				c.info.Types[x] = ret
				return ret
			case 0:
				c.add(diagAt("DTE0101", x.Span, "pipeline has no matching overload for call to "+id.Name))
				return nil
			default:
				c.add(diagAt("DTE0102", x.Span, "pipeline ambiguous overload for call to "+id.Name))
				return nil
			}
		}

		// Named-args pipeline: synthesize [lhs]+args and run per-candidate mapping
		synth := make([]ast.CallArg, 0, 1+len(base))
		synth = append(synth, ast.CallArg{Expr: &ast.Ident{Name: "<pipe>", Span: x.Lhs.SpanOf()}})
		synth = append(synth, base...)

		var exact []*FuncCand
		for _, cand := range set.Cands {
			if cand.Type == nil || len(cand.Type.Params) == 0 {
				continue
			}
			vec, ok := c.canonicalizeForCandidate(cand, synth)
			if !ok {
				continue
			}
			vec[0] = lhsT // force first param to be lhsT
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
			ret := chosen.Type.Ret
			c.info.Types[call.Callee] = chosen.Type
			c.info.Types[x] = ret
			return ret
		case 0:
			c.add(diagAt("DTE0101", x.Span, "pipeline has no matching overload for call to "+id.Name))
			return nil
		default:
			c.add(diagAt("DTE0102", x.Span, "pipeline ambiguous overload for call to "+id.Name))
			return nil
		}

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

		c.add(diagAt("DTE0004", x.Span, "unsupported 'in' operands"))
		return nil

	default:
		return nil
	}
}

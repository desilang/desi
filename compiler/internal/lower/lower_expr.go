package lower

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/types"
)

// lowerExpr converts surface AST expressions to HIR values (Tier-0).
// NOTE: This stays intentionally minimal: enough for async demos, arena alloc,
//
//	prelude stubs, and (now) list comprehensions ➜ list_push calls.
func (ls *lowerState) lowerExpr(e ast.Expr) hir.Value {
	switch x := e.(type) {
	case *ast.IntLit:
		return hir.ConstInt{Text: x.Text}
	case *ast.FloatLit:
		return hir.ConstFloat{Text: x.Text}
	case *ast.BoolLit:
		return hir.ConstBool{Value: x.Value}

	case *ast.NoneLit:
		// Lower none as null pointer (0)
		return &hir.ConstInt{Text: "0"}

	case *ast.StrLit:
		// If Value is populated (F-string part), use it
		if x.Value != "" {
			// Unescape the string to process escape sequences like \n, \t, etc.
			return hir.ConstStr{Text: unescapeString(x.Value)}
		}
		// Otherwise, extract from source
		if ls.src != nil {
			text, ok := scanStringLiteral(ls.src, x.Span.Start.Line, x.Span.Start.Col, x.Long)
			if ok {
				// Unescape the extracted string as well
				return hir.ConstStr{Text: unescapeString(text)}
			}
		}
		// Fallback to placeholder if source unavailable
		return hir.ConstStr{Text: "<lit>"}
	case *ast.FString:
		// F-string: use asprintf for formatting
		// 1. Allocate buffer for result
		bufPtr := ls.b.FreshTemp("fstr_buf")
		ls.b.Emit(&hir.Alloca{Type: "ptr", Dst: bufPtr})

		// 2. Build format string and arguments
		var fmtBuilder strings.Builder
		var args []hir.Value
		args = append(args, bufPtr) // First arg is &bufPtr

		for _, part := range x.Parts {
			switch p := part.(type) {
			case *ast.StrLit:
				// For F-string parts, use the Value field directly
				if p.Value != "" {
					// First unescape escape sequences like \n, \t, etc.
					text := unescapeString(p.Value)
					// Then unescape {{ and }}
					unescaped := strings.ReplaceAll(text, "{{", "{")
					unescaped = strings.ReplaceAll(unescaped, "}}", "}")
					// Escape % for printf
					escaped := strings.ReplaceAll(unescaped, "%", "%%")
					fmtBuilder.WriteString(escaped)
				}
			default:
				// Expression - lower it and add format specifier
				val := ls.lowerExpr(p)
				if ls.info != nil {
					typ := ls.info.Types[p]
					if types.Equal(typ, types.Int) {
						fmtBuilder.WriteString("%lld")
					} else if types.Equal(typ, types.Str) {
						fmtBuilder.WriteString("%s")
					} else if types.Equal(typ, types.Float) {
						fmtBuilder.WriteString("%f")
					} else if types.Equal(typ, types.Bool) {
						fmtBuilder.WriteString("%s")
						// Convert bool to string pointer using runtime helper
						// We need to emit a call: bool_to_cstring(val) -> ptr
						res := ls.b.FreshTemp("bool_str")
						ls.b.Emit(&hir.Call{Dst: res, Fn: "bool_to_cstring", Args: []hir.Value{val}})
						val = res
					} else {
						fmtBuilder.WriteString("<?>")
					}
				} else {
					fmtBuilder.WriteString("%s")
				}
				args = append(args, val)
			}
		}

		// 3. Create format string constant and build final args
		fmtStr := hir.ConstStr{Text: fmtBuilder.String()}
		finalArgs := make([]hir.Value, 0, len(args)+1)
		finalArgs = append(finalArgs, args[0])     // bufPtr
		finalArgs = append(finalArgs, fmtStr)      // format string
		finalArgs = append(finalArgs, args[1:]...) // remaining args

		// 4. Call asprintf
		ls.b.Emit(&hir.Call{Fn: "asprintf", Args: finalArgs})

		// 5. Load result from buffer
		res := ls.b.FreshTemp("fstr_res")
		ls.b.Emit(&hir.Load{Type: "ptr", Src: bufPtr, Dst: res})

		return res
	case *ast.TupleLit:
		// Tuples are heap-allocated using arena allocator to support returning from generic functions.
		// For Tier-0 Generics (Type Erasure), ALL tuple elements are boxed to 'ptr'.
		// This ensures layout compatibility between (T, T) -> {ptr, ptr} and (int, int).
		var elemTypes []string
		for range x.Elems {
			elemTypes = append(elemTypes, "ptr")
		}
		structType := "{" + strings.Join(elemTypes, ", ") + "}"

		// Calculate struct size (ptr = 8 bytes on 64-bit, so N elements = N * 8)
		structSize := len(x.Elems) * 8

		// Allocate on heap using malloc
		dst := ls.b.FreshTemp("tuple_ptr")
		sizeVal := hir.ConstInt{Text: fmt.Sprintf("%d", structSize), Type: "i64"}
		ls.b.Emit(&hir.Call{Dst: dst, Fn: "malloc", Args: []hir.Value{sizeVal}, Type: "ptr"})

		// Store elements
		for i, e := range x.Elems {
			val := ls.lowerExpr(e)

			// Box if necessary (allocate + store for primitives)
			valType := "ptr" // default
			if ls.info != nil {
				if t := ls.info.Types[e]; t != nil {
					valType = lowerType(t)
				}
			}

			var boxedVal hir.Value = val
			if valType != "ptr" && valType != "void" {
				// Allocate storage for the value on heap
				boxPtr := ls.b.FreshTemp("elem_box_ptr")
				elemSize := hir.ConstInt{Text: "8", Type: "i64"} // conservative: always 8 bytes
				if valType == "i32" || valType == "i1" {
					elemSize = hir.ConstInt{Text: "4", Type: "i64"}
				}
				ls.b.Emit(&hir.Call{Dst: boxPtr, Fn: "malloc", Args: []hir.Value{elemSize}, Type: "ptr"})
				// Store the value
				ls.b.Emit(&hir.Store{Dst: boxPtr, Val: val})
				boxedVal = boxPtr
			}

			// GEP
			fieldPtr := ls.b.FreshTemp("tuple_field")
			ls.b.Emit(&hir.GetElementPtr{
				Type:    structType,
				Base:    dst,
				Indices: []hir.Value{hir.ConstInt{Text: "0"}, hir.ConstInt{Text: fmt.Sprintf("%d", i)}},
				Dst:     fieldPtr,
			})

			// Store
			ls.b.Emit(&hir.Store{Dst: fieldPtr, Val: boxedVal})
		}
		return dst

	case *ast.SliceExpr:
		// list[start:end] -> list_slice(list, start, end)
		list := ls.lowerExpr(x.X)

		// Start index (default 0)
		var start hir.Value
		if x.I != nil {
			val := ls.lowerExpr(x.I)
			// Cast i32 to i64 for runtime call
			start = ls.b.FreshTemp("start_i64")
			ls.b.Emit(&hir.Cast{Dst: start.(hir.Temp), Src: val, Type: "i64"})
		} else {
			start = hir.ConstInt{Text: "0", Type: "i64"}
		}

		// End index (default len(list))
		var end hir.Value
		if x.J != nil {
			val := ls.lowerExpr(x.J)
			// Cast i32 to i64 for runtime call
			end = ls.b.FreshTemp("end_i64")
			ls.b.Emit(&hir.Cast{Dst: end.(hir.Temp), Src: val, Type: "i64"})
		} else {
			// Call list_len -> i64
			end = ls.b.FreshTemp("len_i64")
			ls.b.Emit(&hir.Call{Dst: end.(hir.Temp), Fn: "list_len", Args: []hir.Value{list}})
		}

		// Call list_slice
		res := ls.b.FreshTemp("slice")
		ls.b.Emit(&hir.Call{Dst: res, Fn: "list_slice", Args: []hir.Value{list, start, end}})
		return res

	case *ast.IndexExpr:
		lhsType := ls.info.Types[x.X]
		if tup, ok := lhsType.(*types.Tuple); ok {
			base := ls.lowerExpr(x.X) // pointer to tuple
			idxLit, _ := x.Idx.(*ast.IntLit)
			idx, _ := strconv.Atoi(idxLit.Text)

			// Reconstruct struct type string for GEP (all ptrs)
			var elemTypes []string
			for range tup.Elems {
				elemTypes = append(elemTypes, "ptr")
			}
			structType := "{" + strings.Join(elemTypes, ", ") + "}"

			// GEP
			fieldPtr := ls.b.FreshTemp("tuple_elem_ptr")
			ls.b.Emit(&hir.GetElementPtr{
				Type:    structType,
				Base:    base,
				Indices: []hir.Value{hir.ConstInt{Text: "0"}, hir.ConstInt{Text: fmt.Sprintf("%d", idx)}},
				Dst:     fieldPtr,
			})

			// Load ptr
			dstPtr := ls.b.FreshTemp("tuple_elem_ptr_val")
			ls.b.Emit(&hir.Load{
				Type: "ptr",
				Src:  fieldPtr,
				Dst:  dstPtr,
			})

			// Unbox if necessary (load from pointer for primitives)
			targetType := lowerType(tup.Elems[idx])
			if targetType != "ptr" && targetType != "void" {
				// dstPtr points to allocated storage containing the primitive value
				// Load the actual value
				unboxed := ls.b.FreshTemp("unboxed")
				ls.b.Emit(&hir.Load{
					Type: targetType,
					Src:  dstPtr,
					Dst:  unboxed,
				})
				return unboxed
			}

			return dstPtr
		}

		// Handle list indexing: list[index]
		if listType, ok := lhsType.(*types.List); ok {
			list := ls.lowerExpr(x.X)
			index := ls.lowerExpr(x.Idx)

			// list_get returns void* (ptr)
			ptrResult := ls.b.FreshTemp("elem_ptr")
			ls.b.Emit(&hir.Call{Dst: ptrResult, Fn: "list_get", Args: []hir.Value{list, index}})

			// Unbox if element type is primitive
			// Check if we need to convert ptr -> int/bool/float
			needsUnboxing := false
			targetType := "i32" // default

			if listType.Elem != nil {
				switch listType.Elem.String() {
				case "int":
					needsUnboxing = true
					targetType = "i32"
				case "bool":
					needsUnboxing = true
					targetType = "i1"
				case "float":
					needsUnboxing = true
					targetType = "double"
					// pointers (str, list, dict, etc.) don't need unboxing
				}
			}

			if needsUnboxing {
				// Cast ptr back to primitive type (ptrtoint)
				dst := ls.b.FreshTemp("elem")
				ls.b.Emit(&hir.Cast{Dst: dst, Src: ptrResult, Type: targetType})
				return dst
			}

			// For pointer types, return as-is
			return ptrResult
		}
		return hir.Var{Name: "<index_expr>"}

	case *ast.Ident:
		// Check if this is a pattern binding variable first
		if val, ok := ls.matchLocals[x.Name]; ok {
			return val
		}

		// If this is a mutable variable, emit a Load instruction
		if ls.isMutable(x.Name) {
			dst := ls.b.FreshTemp("load")

			// Determine type
			loadType := "i32" // default
			if ls.info != nil {
				if sym := ls.info.Idents[x]; sym != nil {
					loadType = lowerType(sym.Type)
				}
			}

			ls.b.Emit(&hir.Load{
				Type: loadType,
				Src:  hir.Var{Name: x.Name},
				Dst:  dst,
			})
			return dst
		}
		return hir.Var{Name: x.Name}

	case *ast.UnaryExpr:
		// Await (async) is the only unary we lower in Tier-0.
		if x.Op == "await" {
			// await <expr>
			dst := ls.b.FreshTemp("await")
			fut := ls.lowerExpr(x.X)
			ls.b.Emit(&hir.Await{Dst: dst, Fut: fut})
			return dst
		} else if x.Op == "-" {
			// Unary minus: 0 - x
			val := ls.lowerExpr(x.X)
			dst := ls.b.FreshTemp("neg")

			// Determine type
			typ := "i64" // default
			if ls.info != nil {
				if t := ls.info.Types[x]; t != nil {
					typ = lowerType(t)
				}
			}

			var zero hir.Value
			if typ == "double" || typ == "float" {
				zero = hir.ConstFloat{Text: "0.0"}
				// BinaryOp lowering handles operator mapping, but we might need explicit opcode if we want fsub
				// Actually, BinaryOp lowering maps "-" to "sub". We need to update BinaryOp lowering to handle floats too!
				// For now, let's assume BinaryOp lowering will be fixed to handle floats.
				// Wait, BinaryOp lowering maps "-" to "sub" unconditionally.
			} else {
				zero = hir.ConstInt{Text: "0"}
			}

			ls.b.Emit(&hir.BinaryOp{
				Op:   "-",
				LHS:  zero,
				RHS:  val,
				Dst:  dst,
				Type: typ,
			})
			return dst
		} else if x.Op == "not" || x.Op == "!" {
			// Logical not: x ^ 1 (xor with true)
			val := ls.lowerExpr(x.X)
			dst := ls.b.FreshTemp("not")
			ls.b.Emit(&hir.BinaryOp{
				Op:   "==",
				LHS:  val,
				RHS:  hir.ConstBool{Value: false}, // x == false is equivalent to not x
				Dst:  dst,
				Type: "i1",
			})
			return dst
		}

		// Unknown unary: just print-through for now.
		return hir.Var{Name: fmt.Sprintf("unary(%s …)", x.Op)}

	case *ast.CallExpr:
		return ls.lowerCall(x)

	case *ast.FieldExpr:
		base := ls.lowerExpr(x.X)
		name := x.Name.Name

		// Check if base is a struct, class, or generic instance
		var fields []types.Field
		baseType := ls.info.Types[x.X]

		if baseType == nil {
			if id, ok := x.X.(*ast.Ident); ok {
				if sym := ls.info.Idents[id]; sym != nil {
					baseType = sym.Type
				}
			}
		}

		if s, ok := baseType.(*types.Struct); ok {
			fields = s.Fields
		} else if c, ok := baseType.(*types.Class); ok {
			fields = c.Fields
		} else if g, ok := baseType.(*types.Generic); ok {
			if s, ok := g.Base.(*types.Struct); ok {
				fields = s.Fields
			} else if c, ok := g.Base.(*types.Class); ok {
				fields = c.Fields
			}
		}

		if fields != nil {
			// Find field index and offset
			idx := -1
			offset := 0
			for i, f := range fields {
				if f.Name == name {
					idx = i
					break
				}
				offset += getSize(f.Type)
			}

			if idx != -1 {
				// Emit GEP + Load
				fieldPtr := ls.b.FreshTemp("field_ptr")
				ls.b.Emit(&hir.GetElementPtr{
					Type:    "i8", // struct is i8 array
					Base:    base,
					Indices: []hir.Value{hir.ConstInt{Text: fmt.Sprintf("%d", offset)}},
					Dst:     fieldPtr,
				})

				dst := ls.b.FreshTemp("field_val")
				// Determine field type for Load (storage type)
				storageType := lowerType(fields[idx].Type)

				ls.b.Emit(&hir.Load{
					Type:     storageType,
					Src:      fieldPtr,
					Dst:      dst,
					DesiType: fields[idx].Type,
				})

				// Check if unboxing is needed (Generic T -> primitive)
				// If storage is ptr (from T) but expression type is primitive (e.g. int)
				exprType := ls.info.Types[x]
				targetType := lowerType(exprType)

				if storageType == "ptr" && targetType != "ptr" && targetType != "void" {
					// Unbox: ptr -> targetType
					unboxed := ls.b.FreshTemp("unboxed")
					ls.b.Emit(&hir.Cast{Dst: unboxed, Src: dst, Type: targetType})
					return unboxed
				}

				return dst
			}
		}

		// Check if this is a property access on a class
		if cls, ok := baseType.(*types.Class); ok {
			// Check if it's a property
			if _, ok := cls.Properties[name]; ok {
				// Property access should be lowered as a method call
				mangledName := fmt.Sprintf("%s_%s", cls.Name, name)
				dst := ls.b.FreshTemp("prop")
				// Properties are getters with just self parameter
				ls.b.Emit(&hir.Call{Dst: dst, Fn: mangledName, Args: []hir.Value{base}})
				return dst
			}
		}

		// Fallback for methods (dict/set) or unknown types
		dst := ls.b.FreshTemp("field")
		ls.b.Emit(&hir.Call{Dst: dst, Fn: "get.field." + name, Args: []hir.Value{base}})
		return dst

	case *ast.ListLit:
		// Create new list
		res := ls.b.FreshTemp("list")
		ls.b.Emit(&hir.Call{Dst: res, Fn: "list_new", Args: []hir.Value{}})

		// Append elements
		for _, e := range x.Elems {
			val := ls.lowerExpr(e)

			// Cast to ptr for generic storage (void*)
			// This handles both pointers (bitcast) and integers (inttoptr)
			valPtr := ls.b.FreshTemp("val_ptr")
			ls.b.Emit(&hir.Cast{Dst: valPtr, Src: val, Type: "ptr"})

			ls.b.Emit(&hir.Call{Fn: "list_append", Args: []hir.Value{res, valPtr}})
		}
		return res

	case *ast.DictLit:
		return ls.lowerDictLit(x)

	case *ast.SetLit:
		if ls.info != nil {
			if t, ok := ls.info.Types[x].(*types.Set); ok {
				return ls.lowerSetLit(x, t)
			}
		}
		return hir.Var{Name: "<set_lit_error>"}

	case *ast.ListComp:
		return ls.lowerListComp(x)

	case *ast.BinaryExpr:
		// Check for operator overloading
		if ls.info != nil {
			if _, ok := ls.info.BinOpOverloads[x]; ok {
				// Lower operands
				lhs := ls.lowerExpr(x.Lhs)
				rhs := ls.lowerExpr(x.Rhs)

				// Get class type from LHS
				lhsType := ls.info.Types[x.Lhs]
				if cls, ok := lhsType.(*types.Class); ok {
					var dunder string
					switch x.Op {
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

					// Handle != fallback to !__eq__
					negate := false
					if x.Op == "!=" && dunder == "__ne__" {
						// Check if Class___ne__ exists
						if _, hasNe := cls.Dunders["__ne__"]; !hasNe {
							dunder = "__eq__"
							negate = true
						}
					}

					fnName := cls.Name + "_" + dunder
					dst := ls.b.FreshTemp("op_call")
					ls.b.Emit(&hir.Call{
						Dst:  dst,
						Fn:   fnName,
						Args: []hir.Value{lhs, rhs},
					})

					if negate {
						// Negate the result (i32 bool)
						negDst := ls.b.FreshTemp("ne_res")
						ls.b.Emit(&hir.BinaryOp{
							Op:   "==",
							LHS:  dst,
							RHS:  hir.ConstInt{Text: "0"}, // false as i32
							Dst:  negDst,
							Type: "i32", // bool is i32 in LLVM
						})
						return negDst
					}

					return dst
				}
			}
		}

		// Lower binary expressions: arithmetic (+, -, *, /, %, **) and comparisons (<, >, <=, >=, ==, !=)
		lhs := ls.lowerExpr(x.Lhs)
		rhs := ls.lowerExpr(x.Rhs)

		// Determine result type based on operation
		var resultType string
		if x.Op == "==" || x.Op == "!=" || x.Op == "<" || x.Op == ">" || x.Op == "<=" || x.Op == ">=" || x.Op == "and" || x.Op == "or" {
			// Comparison and logical operations return boolean (i1)
			resultType = "i1"
		} else {
			// Arithmetic operations preserve operand type
			if ls.info != nil {
				if typ := ls.info.Types[x]; typ != nil {
					resultType = lowerType(typ)
				} else {
					resultType = "i32" // default fallback
				}
			} else {
				resultType = "i32" // default fallback
			}
		}

		dst := ls.b.FreshTemp("binop")
		ls.b.Emit(&hir.BinaryOp{
			Op:   x.Op,
			LHS:  lhs,
			RHS:  rhs,
			Dst:  dst,
			Type: resultType,
		})

		// If string concatenation, track result for cleanup
		if x.Op == "+" && resultType == "ptr" {
			// Check if it's actually a string type
			isStr := false
			if ls.info != nil {
				if t, ok := ls.info.Types[x]; ok && types.Equal(t, types.Str) {
					isStr = true
				}
			}
			if isStr {
				ls.addTempDrop(dst.Name)
			}
		}

		return dst

	case *ast.MatchExpr:
		return ls.lowerMatchExpr(x)

	default:
		// Print-through placeholder for anything not wired yet.
		return hir.Var{Name: fmt.Sprintf("<expr:%T>", e)}
	}
}

// lowerListComp builds a tiny HIR shape for list comprehensions:
//
//	let %res
//	call list_push(%res, <elem>)
//
// Returns %res as the value of the comprehension.
//
// Tier-0 note: This is a compile-only skeleton. We don't yet expand
// the full generator chain; instead, we ensure the result handle exists
// and we append the element once. Later passes can elaborate to real loops.
func (ls *lowerState) lowerListComp(c *ast.ListComp) hir.Value {
	// Result handle
	res := ls.b.FreshTemp("list")
	ls.b.Emit(&hir.Let{Name: res.Name})

	// Element value
	elem := ls.lowerExpr(c.Elem)
	if elem == nil {
		// Be defensive; use a const 0 if lowering produced nothing.
		elem = &hir.ConstInt{Text: "0"}
	}

	// For now: a single append with prelude stub. (Tight loop elab comes next.)
	ls.b.Emit(&hir.Call{Fn: "list_push", Args: []hir.Value{res, elem}})

	return res
}

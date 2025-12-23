package lower

import (
	"fmt"
	"os"
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
	case *ast.DecimalLit:
		// Lower decimal literal to __decimal_new() call
		// Strip the 'd' suffix and pass as string constant
		text := x.Text
		if len(text) > 0 && (text[len(text)-1] == 'd' || text[len(text)-1] == 'D') {
			text = text[:len(text)-1]
		}
		dst := ls.b.FreshTemp("decimal")
		ls.b.Emit(&hir.Call{Dst: dst, Fn: "__decimal_new", Args: []hir.Value{hir.ConstStr{Text: text}}, Type: "ptr"})
		return dst
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

		// First, collect all elements including spread expansions
		type elemInfo struct {
			expr     ast.Expr
			val      hir.Value
			elemType types.T
		}
		var allElems []elemInfo

		for _, e := range x.Elems {
			if spread, ok := e.(*ast.SpreadExpr); ok {
				// Spread expression: extract elements from the tuple
				spreadVal := ls.lowerExpr(spread.X)
				var spreadType *types.Tuple
				if ls.info != nil {
					if t := ls.info.Types[spread.X]; t != nil {
						if tupT, ok := t.(*types.Tuple); ok {
							spreadType = tupT
						}
					}
				}

				if spreadType != nil {
					// Build struct type for spread tuple
					var spreadElemTypes []string
					for range spreadType.Elems {
						spreadElemTypes = append(spreadElemTypes, "ptr")
					}
					spreadStructType := "{" + strings.Join(spreadElemTypes, ", ") + "}"

					// Extract each element from the spread tuple
					for j, elemT := range spreadType.Elems {
						elemPtr := ls.b.FreshTemp(fmt.Sprintf("spread_elem%d_ptr", j))
						ls.b.Emit(&hir.GetElementPtr{
							Type:    spreadStructType,
							Base:    spreadVal,
							Indices: []hir.Value{hir.ConstInt{Text: "0"}, hir.ConstInt{Text: fmt.Sprintf("%d", j)}},
							Dst:     elemPtr,
						})
						elemBoxed := ls.b.FreshTemp(fmt.Sprintf("spread_elem%d_boxed", j))
						ls.b.Emit(&hir.Load{Type: "ptr", Src: elemPtr, Dst: elemBoxed})

						allElems = append(allElems, elemInfo{expr: nil, val: elemBoxed, elemType: elemT})
					}
				}
			} else {
				// Regular element
				val := ls.lowerExpr(e)
				var elemT types.T
				if ls.info != nil {
					elemT = ls.info.Types[e]
				}
				allElems = append(allElems, elemInfo{expr: e, val: val, elemType: elemT})
			}
		}

		// Now build the result tuple
		var elemTypes []string
		for range allElems {
			elemTypes = append(elemTypes, "ptr")
		}
		structType := "{" + strings.Join(elemTypes, ", ") + "}"

		// Calculate struct size (ptr = 8 bytes on 64-bit, so N elements = N * 8)
		structSize := len(allElems) * 8

		// Allocate on heap using malloc
		dst := ls.b.FreshTemp("tuple_ptr")
		sizeVal := hir.ConstInt{Text: fmt.Sprintf("%d", structSize), Type: "i64"}
		ls.b.Emit(&hir.Call{Dst: dst, Fn: "malloc", Args: []hir.Value{sizeVal}, Type: "ptr"})

		// Store elements
		for i, elem := range allElems {
			val := elem.val

			// Box if necessary (allocate + store for primitives)
			valType := "ptr" // default
			if elem.elemType != nil {
				valType = lowerType(elem.elemType)
			}

			var boxedVal hir.Value = val
			// For spread elements, val is already boxed (we loaded the ptr from the spread tuple)
			// For regular elements, we need to box them
			if elem.expr != nil && valType != "ptr" && valType != "void" {
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
		// Check if we're slicing a string vs. a list vs. a tuple
		var slicedType types.T
		if ls.info != nil {
			slicedType = ls.info.Types[x.X]
		}
		isStr := slicedType == types.Str

		// Handle tuple slicing (compile-time)
		if tupType, ok := slicedType.(*types.Tuple); ok {
			base := ls.lowerExpr(x.X)

			// Get compile-time indices (type checker already validated these)
			iVal := 0
			jVal := len(tupType.Elems)
			if x.I != nil {
				if lit, ok := x.I.(*ast.IntLit); ok {
					iVal, _ = strconv.Atoi(lit.Text)
				}
			}
			if x.J != nil {
				if lit, ok := x.J.(*ast.IntLit); ok {
					jVal, _ = strconv.Atoi(lit.Text)
				}
			}

			sliceCount := jVal - iVal
			if sliceCount == 1 {
				// Single element - just return the element value (no tuple)
				elemType := tupType.Elems[iVal]
				llvmElemType := lowerType(elemType)

				// Build source tuple struct type
				var srcElemTypes []string
				for range tupType.Elems {
					srcElemTypes = append(srcElemTypes, "ptr")
				}
				srcStructType := "{" + strings.Join(srcElemTypes, ", ") + "}"

				// Get element from source tuple
				elemPtr := ls.b.FreshTemp("slice_elem_ptr")
				ls.b.Emit(&hir.GetElementPtr{
					Type:    srcStructType,
					Base:    base,
					Indices: []hir.Value{hir.ConstInt{Text: "0"}, hir.ConstInt{Text: fmt.Sprintf("%d", iVal)}},
					Dst:     elemPtr,
				})
				elemBoxed := ls.b.FreshTemp("slice_elem_boxed")
				ls.b.Emit(&hir.Load{Type: "ptr", Src: elemPtr, Dst: elemBoxed})

				// Return unboxed value (or boxed for ptr types)
				if llvmElemType == "ptr" {
					return elemBoxed
				}
				elemVal := ls.b.FreshTemp("slice_elem_val")
				ls.b.Emit(&hir.Load{Type: llvmElemType, Src: elemBoxed, Dst: elemVal})
				return elemVal
			}

			// Multi-element slice - create new tuple
			// Build source tuple struct type
			var srcElemTypes []string
			for range tupType.Elems {
				srcElemTypes = append(srcElemTypes, "ptr")
			}
			srcStructType := "{" + strings.Join(srcElemTypes, ", ") + "}"

			// Build destination tuple struct type
			var dstElemTypes []string
			for i := 0; i < sliceCount; i++ {
				dstElemTypes = append(dstElemTypes, "ptr")
			}
			dstStructType := "{" + strings.Join(dstElemTypes, ", ") + "}"

			// Allocate new tuple
			dstSize := sliceCount * 8
			dst := ls.b.FreshTemp("slice_tuple")
			ls.b.Emit(&hir.Call{Dst: dst, Fn: "malloc", Args: []hir.Value{hir.ConstInt{Text: fmt.Sprintf("%d", dstSize), Type: "i64"}}, Type: "ptr"})

			// Copy elements from source to destination
			for i := 0; i < sliceCount; i++ {
				srcIdx := iVal + i

				// Get boxed ptr from source tuple
				srcPtr := ls.b.FreshTemp("slice_src_ptr")
				ls.b.Emit(&hir.GetElementPtr{
					Type:    srcStructType,
					Base:    base,
					Indices: []hir.Value{hir.ConstInt{Text: "0"}, hir.ConstInt{Text: fmt.Sprintf("%d", srcIdx)}},
					Dst:     srcPtr,
				})
				srcBoxed := ls.b.FreshTemp("slice_src_boxed")
				ls.b.Emit(&hir.Load{Type: "ptr", Src: srcPtr, Dst: srcBoxed})

				// Store in destination tuple
				dstPtr := ls.b.FreshTemp("slice_dst_ptr")
				ls.b.Emit(&hir.GetElementPtr{
					Type:    dstStructType,
					Base:    dst,
					Indices: []hir.Value{hir.ConstInt{Text: "0"}, hir.ConstInt{Text: fmt.Sprintf("%d", i)}},
					Dst:     dstPtr,
				})
				ls.b.Emit(&hir.Store{Dst: dstPtr, Val: srcBoxed})
			}

			return dst
		}

		// Check for custom class slicing via __getslice__
		if ls.info != nil {
			if cls, ok := slicedType.(*types.Class); ok {
				if getSliceFn, found := cls.Dunders["__getslice__"]; found {
					base := ls.lowerExpr(x.X)

					// Start index (default 0)
					var start hir.Value
					if x.I != nil {
						start = ls.lowerExpr(x.I)
					} else {
						start = hir.ConstInt{Text: "0", Type: "i32"}
					}

					// End index (default -1 meaning "to end" or call __len__)
					var end hir.Value
					if x.J != nil {
						end = ls.lowerExpr(x.J)
					} else {
						// Call __len__ to get default end
						if _, hasLen := cls.Dunders["__len__"]; hasLen {
							lenMangledName := fmt.Sprintf("%s___len__", cls.Name)
							end = ls.b.FreshTemp("slice_len")
							ls.b.Emit(&hir.Call{Dst: end.(hir.Temp), Fn: lenMangledName, Args: []hir.Value{base}, Type: "i32"})
						} else {
							// Use sentinel -1 if no __len__
							end = hir.ConstInt{Text: "-1", Type: "i32"}
						}
					}

					// Call __getslice__(self, start, end)
					mangledName := fmt.Sprintf("%s___getslice__", cls.Name)
					res := ls.b.FreshTemp("slice")
					retType := "ptr" // default return type
					if getSliceFn.Ret != nil {
						retType = lowerType(getSliceFn.Ret)
					}
					ls.b.Emit(&hir.Call{Dst: res, Fn: mangledName, Args: []hir.Value{base, start, end}, Type: retType})
					return res
				}
			}
		}

		base := ls.lowerExpr(x.X)

		// Start index (default 0)
		var start hir.Value
		if x.I != nil {
			val := ls.lowerExpr(x.I)
			if isStr {
				// string_substr expects i32
				start = val
			} else {
				// list_slice expects i64
				start = ls.b.FreshTemp("start_i64")
				ls.b.Emit(&hir.Cast{Dst: start.(hir.Temp), Src: val, Type: "i64"})
			}
		} else {
			if isStr {
				start = hir.ConstInt{Text: "0", Type: "i32"}
			} else {
				start = hir.ConstInt{Text: "0", Type: "i64"}
			}
		}

		// End index (default len(x))
		var end hir.Value
		if x.J != nil {
			val := ls.lowerExpr(x.J)
			if isStr {
				// string_substr expects i32
				end = val
			} else {
				// list_slice expects i64
				end = ls.b.FreshTemp("end_i64")
				ls.b.Emit(&hir.Cast{Dst: end.(hir.Temp), Src: val, Type: "i64"})
			}
		} else {
			if isStr {
				// Call string_len -> i32
				end = ls.b.FreshTemp("strlen")
				ls.b.Emit(&hir.Call{Dst: end.(hir.Temp), Fn: "string_len", Args: []hir.Value{base}, Type: "i32"})
			} else {
				// Call list_len -> i64
				end = ls.b.FreshTemp("len_i64")
				ls.b.Emit(&hir.Call{Dst: end.(hir.Temp), Fn: "list_len", Args: []hir.Value{base}})
			}
		}

		res := ls.b.FreshTemp("slice")

		// Check if step (K) is provided
		if x.K != nil {
			// Step slicing: use list_slice_step or string_slice_step
			stepVal := ls.lowerExpr(x.K)

			// For step slicing, use sentinel values for omitted start/end
			// INT64_MAX (9223372036854775807) = use default based on step sign
			var stepStart, stepEnd hir.Value
			if x.I != nil {
				val := ls.lowerExpr(x.I)
				if isStr {
					stepStart = val
				} else {
					stepStart = ls.b.FreshTemp("stepstart_i64")
					ls.b.Emit(&hir.Cast{Dst: stepStart.(hir.Temp), Src: val, Type: "i64"})
				}
			} else {
				// Sentinel value: INT64_MAX means "use default"
				if isStr {
					stepStart = hir.ConstInt{Text: "2147483647", Type: "i32"} // INT32_MAX
				} else {
					stepStart = hir.ConstInt{Text: "9223372036854775807", Type: "i64"}
				}
			}

			if x.J != nil {
				val := ls.lowerExpr(x.J)
				if isStr {
					stepEnd = val
				} else {
					stepEnd = ls.b.FreshTemp("stepend_i64")
					ls.b.Emit(&hir.Cast{Dst: stepEnd.(hir.Temp), Src: val, Type: "i64"})
				}
			} else {
				// Sentinel value: INT64_MIN means "use default"
				if isStr {
					stepEnd = hir.ConstInt{Text: "-2147483648", Type: "i32"} // INT32_MIN
				} else {
					stepEnd = hir.ConstInt{Text: "-9223372036854775808", Type: "i64"}
				}
			}

			if isStr {
				// string_slice_step(str, start, end, step)
				ls.b.Emit(&hir.Call{Dst: res, Fn: "string_slice_step", Args: []hir.Value{base, stepStart, stepEnd, stepVal}, Type: "ptr"})
			} else {
				// list_slice_step(list, start, end, step)
				step64 := ls.b.FreshTemp("step_i64")
				ls.b.Emit(&hir.Cast{Dst: step64, Src: stepVal, Type: "i64"})
				ls.b.Emit(&hir.Call{Dst: res, Fn: "list_slice_step", Args: []hir.Value{base, stepStart, stepEnd, step64}, Type: "ptr"})
			}
		} else {
			// No step - use regular slice functions
			if isStr {
				ls.b.Emit(&hir.Call{Dst: res, Fn: "string_slice", Args: []hir.Value{base, start, end}, Type: "ptr"})
			} else {
				ls.b.Emit(&hir.Call{Dst: res, Fn: "list_slice", Args: []hir.Value{base, start, end}})
			}
		}
		return res

	case *ast.IndexExpr:
		lhsType := ls.info.Types[x.X]

		// Handle custom classes with __getitem__ dunder
		if cls, ok := lhsType.(*types.Class); ok {
			if _, found := cls.Dunders["__getitem__"]; found {
				// Desugar obj[idx] to obj.__getitem__(idx)
				obj := ls.lowerExpr(x.X)
				idx := ls.lowerExpr(x.Idx)

				// Call __getitem__ method
				mangledName := fmt.Sprintf("%s___getitem__", cls.Name)
				dst := ls.b.FreshTemp("getitem")

				// Determine return type
				retType := "i32" // default
				if getitem := cls.Dunders["__getitem__"]; getitem != nil && getitem.Ret != nil {
					retType = lowerType(getitem.Ret)
				}

				ls.b.Emit(&hir.Call{
					Dst:  dst,
					Fn:   mangledName,
					Args: []hir.Value{obj, idx},
					Type: retType,
				})
				return dst
			}
		}

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

			// Sign-extend index to i64 for proper negative index handling
			index64 := ls.b.FreshTemp("index_i64")
			ls.b.Emit(&hir.Cast{Dst: index64, Src: index, Type: "i64"})

			// list_get returns void* (ptr)
			ptrResult := ls.b.FreshTemp("elem_ptr")
			ls.b.Emit(&hir.Call{Dst: ptrResult, Fn: "list_get", Args: []hir.Value{list, index64}})

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

		// If lhsType is nil, try to infer from FieldExpr
		if lhsType == nil {
			if fieldExpr, ok := x.X.(*ast.FieldExpr); ok {
				// Try to get the type of the field
				if ls.info != nil {
					if baseType := ls.info.Types[fieldExpr.X]; baseType != nil {
						if cls, ok := baseType.(*types.Class); ok {
							// Look up the field in the class
							fieldName := fieldExpr.Name.Name
							for _, field := range cls.Fields {
								if field.Name == fieldName {
									lhsType = field.Type
									break
								}
							}
						}
					}
				}
			}
		}

		// Retry list indexing with inferred type
		if listType, ok := lhsType.(*types.List); ok {
			list := ls.lowerExpr(x.X)
			index := ls.lowerExpr(x.Idx)

			// list_get returns void* (ptr)
			ptrResult := ls.b.FreshTemp("elem_ptr")
			ls.b.Emit(&hir.Call{Dst: ptrResult, Fn: "list_get", Args: []hir.Value{list, index}})

			// Unbox if element type is primitive
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
				}
			}

			if needsUnboxing {
				dst := ls.b.FreshTemp("elem")
				ls.b.Emit(&hir.Cast{Dst: dst, Src: ptrResult, Type: targetType})
				return dst
			}

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
		// 1. Check for dunder methods on custom types
		if ls.info != nil {
			if t := ls.info.Types[x.X]; t != nil {
				if cls, ok := t.(*types.Class); ok {
					var method string
					switch x.Op {
					case "-":
						method = "__neg__"
					case "+":
						method = "__pos__"
					case "~":
						method = "__invert__"
					}

					if method != "" {
						// Check if method exists (checker already verified this, but good to be safe)
						// We need to find the method to get its mangled name or just use lowerCall logic?
						// Actually, we can construct a CallExpr and lower it, OR manually emit the call.
						// Constructing a CallExpr is easier as it reuses lowerCall logic (mangling, self passing, etc.)
						// But we are in lowerExpr, we can just call ls.lowerMethodCall?
						// ls.lowerMethodCall takes a CallExpr.
						// Let's manually construct the HIR call.

						// Resolve method to get mangled name
						// Inline mangling: ClassName_MethodName
						mangled := fmt.Sprintf("%s_%s", cls.Name, method)

						// Lower receiver
						recv := ls.lowerExpr(x.X)

						// Emit Call
						dst := ls.b.FreshTemp("unary_res")

						// We need the return type for the call instruction
						retType := "void"
						// Try to get return type from checker info
						if retT := ls.info.Types[x]; retT != nil {
							retType = lowerType(retT)
						}

						ls.b.Emit(&hir.Call{
							Dst:  dst,
							Fn:   mangled,
							Args: []hir.Value{recv},
							Type: retType,
						})
						return dst
					}
				}
			}
		}

		// 2. Primitive / Builtin handling
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
		} else if x.Op == "+" {
			// Unary plus: +x -> x (no-op for primitives)
			return ls.lowerExpr(x.X)
		} else if x.Op == "~" {
			// Bitwise NOT: ~x -> x ^ -1
			val := ls.lowerExpr(x.X)
			dst := ls.b.FreshTemp("not")

			// Assume integer for now (checker enforces it)
			typ := "i64"
			if ls.info != nil {
				if t := ls.info.Types[x]; t != nil {
					typ = lowerType(t)
				}
			}

			ls.b.Emit(&hir.BinaryOp{
				Op:   "^",
				LHS:  val,
				RHS:  hir.ConstInt{Text: "-1"},
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

	case *ast.TryExpr:
		// ? operator: unwrap Result/Option by loading payload directly
		// Full semantics (match + early return on Err/Nothing) can be added later
		inner := ls.lowerExpr(x.X)

		// Get payload ptr at offset 4 (same as unwrap() method)
		payloadPtrSlot := ls.b.FreshTemp("payload_ptr_slot")
		ls.b.Emit(&hir.GetElementPtr{
			Type:    "i8",
			Base:    inner,
			Indices: []hir.Value{hir.ConstInt{Text: "4"}},
			Dst:     payloadPtrSlot,
		})
		payloadPtr := ls.b.FreshTemp("payload_ptr")
		ls.b.Emit(&hir.Load{Type: "ptr", Src: payloadPtrSlot, Dst: payloadPtr})

		// Load value from payload ptr
		// Get the element type from the Result/Option generic type
		var elemType types.T
		if ls.info != nil {
			if recvType := ls.info.Types[x.X]; recvType != nil {
				if g, ok := recvType.(*types.Generic); ok {
					if len(g.Args) > 0 {
						elemType = g.Args[0] // T from Result[T,E] or Option[T]
					}
				}
			}
		}

		valType := "ptr" // default
		if elemType != nil {
			valType = lowerType(elemType)
		}

		val := ls.b.FreshTemp("try_val")
		ls.b.Emit(&hir.Load{Type: valType, Src: payloadPtr, Dst: val})
		return val

	case *ast.IsExpr:
		// 'is' operator: identity comparison or pattern matching
		lhs := ls.lowerExpr(x.X)
		dst := ls.b.FreshTemp("is_result")

		// Get the type of the left-hand side
		var lhsType types.T
		if ls.info != nil {
			lhsType = ls.info.Types[x.X]
		}

		// Check if we're matching against none/Nothing for Option types
		// 'none' can be parsed as either Ident("none") or NoneLit
		isNonePattern := false
		if ident, ok := x.Pattern.(*ast.Ident); ok && (ident.Name == "none" || ident.Name == "Nothing") {
			isNonePattern = true
		}
		if _, ok := x.Pattern.(*ast.NoneLit); ok {
			isNonePattern = true
		}

		if isNonePattern {
			// Check if LHS is Option type using helper
			isOption := lhsType != nil && types.IsOption(lhsType)

			if isOption {
				// Option: check tag == 1 (Nothing)
				tagPtr := ls.b.FreshTemp("tag_ptr")
				ls.b.Emit(&hir.GetElementPtr{
					Type:    "i8",
					Base:    lhs,
					Indices: []hir.Value{hir.ConstInt{Text: "0"}},
					Dst:     tagPtr,
				})
				tag := ls.b.FreshTemp("tag")
				ls.b.Emit(&hir.Load{Type: "i32", Src: tagPtr, Dst: tag})

				ls.b.Emit(&hir.BinaryOp{
					Op:   "==",
					LHS:  tag,
					RHS:  hir.ConstInt{Text: "1"},
					Dst:  dst,
					Type: "i1",
				})
			} else {
				// Non-Option: 'is none' means compare to the none value
				// For non-Option types, lower 'none' as expression
				rhs := ls.lowerExpr(x.Pattern)
				ls.b.Emit(&hir.BinaryOp{
					Op:   "==",
					LHS:  lhs,
					RHS:  rhs,
					Dst:  dst,
					Type: "i1",
				})
			}
		} else if call, ok := x.Pattern.(*ast.CallExpr); ok {
			// Pattern like Some(x) or Ok(v) - check variant tag
			if callee, ok := call.Callee.(*ast.Ident); ok {
				switch callee.Name {
				case "Some":
					// Check tag == 0 for Option.Some
					tagPtr := ls.b.FreshTemp("tag_ptr")
					ls.b.Emit(&hir.GetElementPtr{
						Type:    "i8",
						Base:    lhs,
						Indices: []hir.Value{hir.ConstInt{Text: "0"}},
						Dst:     tagPtr,
					})
					tag := ls.b.FreshTemp("tag")
					ls.b.Emit(&hir.Load{Type: "i32", Src: tagPtr, Dst: tag})

					ls.b.Emit(&hir.BinaryOp{
						Op:   "==",
						LHS:  tag,
						RHS:  hir.ConstInt{Text: "0"},
						Dst:  dst,
						Type: "i1",
					})
				case "Ok":
					// Check tag == 0 for Result.Ok
					tagPtr := ls.b.FreshTemp("tag_ptr")
					ls.b.Emit(&hir.GetElementPtr{
						Type:    "i8",
						Base:    lhs,
						Indices: []hir.Value{hir.ConstInt{Text: "0"}},
						Dst:     tagPtr,
					})
					tag := ls.b.FreshTemp("tag")
					ls.b.Emit(&hir.Load{Type: "i32", Src: tagPtr, Dst: tag})

					ls.b.Emit(&hir.BinaryOp{
						Op:   "==",
						LHS:  tag,
						RHS:  hir.ConstInt{Text: "0"},
						Dst:  dst,
						Type: "i1",
					})
				case "Err":
					// Check tag == 1 for Result.Err
					tagPtr := ls.b.FreshTemp("tag_ptr")
					ls.b.Emit(&hir.GetElementPtr{
						Type:    "i8",
						Base:    lhs,
						Indices: []hir.Value{hir.ConstInt{Text: "0"}},
						Dst:     tagPtr,
					})
					tag := ls.b.FreshTemp("tag")
					ls.b.Emit(&hir.Load{Type: "i32", Src: tagPtr, Dst: tag})

					ls.b.Emit(&hir.BinaryOp{
						Op:   "==",
						LHS:  tag,
						RHS:  hir.ConstInt{Text: "1"},
						Dst:  dst,
						Type: "i1",
					})
				default:
					// Unknown pattern - fallback to identity
					rhs := ls.lowerExpr(x.Pattern)
					ls.b.Emit(&hir.BinaryOp{
						Op:   "==",
						LHS:  lhs,
						RHS:  rhs,
						Dst:  dst,
						Type: "i1",
					})
				}
			} else {
				// FieldExpr callee like Option.Some
				rhs := ls.lowerExpr(x.Pattern)
				ls.b.Emit(&hir.BinaryOp{
					Op:   "==",
					LHS:  lhs,
					RHS:  rhs,
					Dst:  dst,
					Type: "i1",
				})
			}
		} else {
			// General identity comparison: compare values/pointers
			rhs := ls.lowerExpr(x.Pattern)
			ls.b.Emit(&hir.BinaryOp{
				Op:   "==",
				LHS:  lhs,
				RHS:  rhs,
				Dst:  dst,
				Type: "i1",
			})
		}

		// Handle negation ('is not')
		if x.Negated {
			negDst := ls.b.FreshTemp("is_not_result")
			ls.b.Emit(&hir.BinaryOp{
				Op:   "==",
				LHS:  dst,
				RHS:  hir.ConstBool{Value: false},
				Dst:  negDst,
				Type: "i1",
			})
			return negDst
		}
		return dst

	case *ast.CastExpr:
		// 'as' expression: explicit type cast
		src := ls.lowerExpr(x.X)
		dstType := ls.info.Types[e]

		// Get the LLVM target type
		dstLLVMType := lowerType(dstType)

		// If same type, no-op
		srcType := ls.info.Types[x.X]
		srcLLVMType := lowerType(srcType)
		if srcLLVMType == dstLLVMType {
			return src
		}

		// Emit cast to target type
		dst := ls.b.FreshTemp("cast")
		ls.b.Emit(&hir.Cast{Dst: dst, Src: src, Type: dstLLVMType})
		return dst

	case *ast.CallExpr:
		return ls.lowerCall(x)

	case *ast.FieldExpr:
		// Special handling for Option/Result variants accessed as fields (e.g. Option.Nothing)
		if id, ok := x.X.(*ast.Ident); ok {
			if id.Name == "Option" || id.Name == "Result" {
				// Treat as constructor call (e.g. Option.Nothing())
				// This handles unit variants like Option.Nothing being used as values.
				ctorName := fmt.Sprintf("%s.%s", id.Name, x.Name.Name)
				dst := ls.b.FreshTemp("enum_ctor")
				ls.b.Emit(&hir.Call{Dst: dst, Fn: ctorName, Args: []hir.Value{}, Type: "ptr"})
				return dst
			}
		}

		// Check for Class Constant access (ClassName.CONST)
		if ls.info != nil {
			if id, ok := x.X.(*ast.Ident); ok {
				// Check if base is a Type symbol
				// Note: We need to access check.SymType, but check package might not be imported or exposed?
				// ls.info.Idents returns *check.Symbol.
				// Let's assume we can access Symbol.Kind or check against SymType if imported.
				// Actually, we can just check if sym.Type is a Class and it has the constant.
				if sym := ls.info.Idents[id]; sym != nil {
					if cls, ok := sym.Type.(*types.Class); ok {
						// Check if it's a constant
						// We need to traverse base classes too?
						// The checker already resolved it, but here we need to find the value.
						curr := cls
						for curr != nil {
							if cnst, ok := curr.Constants[x.Name.Name]; ok {
								// Substitute constant value
								if valExpr, ok := cnst.Value.(ast.Expr); ok {
									return ls.lowerExpr(valExpr)
								}
								// Should not happen if checker did its job
								return hir.Var{Name: "<const_error>"}
							}
							curr = curr.Base
						}

						// Check for static field access (ClassName.FIELD)
						curr = cls
						for curr != nil {
							if sf, ok := curr.StaticFields[x.Name.Name]; ok {
								// Emit Load from global: @ClassName_FieldName
								globalName := fmt.Sprintf("%s_%s", cls.Name, x.Name.Name)
								dst := ls.b.FreshTemp("sfload")
								sfType := lowerType(sf.Type)
								ls.b.Emit(&hir.Load{
									Type: sfType,
									Src:  hir.Var{Name: "@" + globalName},
									Dst:  dst,
								})
								return dst
							}
							curr = curr.Base
						}
					}
				}
			}
		}

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

		// Unwrap TypeAlias to access underlying type for field access
		if alias, ok := baseType.(*types.TypeAlias); ok {
			baseType = alias.Target
		}

		// Handle Mutex.lock() method - returns MutexGuard (note: call is handled in CallExpr, this is just accessing method as value)
		if m, ok := baseType.(*types.Mutex); ok {
			_ = m // Mutex field access - currently only .lock() which is a method call, handled in CallExpr
		}

		// Handle MutexGuard.value field - get the protected value
		if g, ok := baseType.(*types.MutexGuard); ok {
			if name == "value" {
				// Call mutex_guard_get(guard) -> void* (pointer to boxed value)
				ptrVal := ls.b.FreshTemp("guard_ptr")
				ls.b.Emit(&hir.Call{Dst: ptrVal, Fn: "mutex_guard_get", Args: []hir.Value{base}, Type: "ptr"})
				// Load the actual value from the boxed pointer
				loadedVal := ls.b.FreshTemp("guard_value")
				loadType := lowerType(g.Inner) // Get the inner type (e.g., i32 for int)
				ls.b.Emit(&hir.Load{Type: loadType, Src: ptrVal, Dst: loadedVal})
				return loadedVal
			}
		}

		if s, ok := baseType.(*types.Struct); ok {
			fields = s.Fields
		} else if c, ok := baseType.(*types.Class); ok {
			fields = c.Fields
		} else if tupType, ok := baseType.(*types.Tuple); ok {
			// Tuple field access: t.0, t.1, etc. or named: t.x, t.y
			idx := -1

			// First, try named field lookup
			if tupType.IsNamed() {
				idx = tupType.FieldIndex(name)
			}

			// Fall back to integer index
			if idx < 0 {
				var err error
				idx, err = strconv.Atoi(name)
				if err != nil || idx < 0 || idx >= len(tupType.Elems) {
					idx = -1
				}
			}

			if idx >= 0 && idx < len(tupType.Elems) {
				elemType := tupType.Elems[idx]
				llvmElemType := lowerType(elemType)

				// Build struct type for GEP (all elements are ptr due to boxing)
				var elemTypes []string
				for range tupType.Elems {
					elemTypes = append(elemTypes, "ptr")
				}
				structType := "{" + strings.Join(elemTypes, ", ") + "}"

				// Get pointer to element
				elemPtr := ls.b.FreshTemp("tuple_elem_ptr")
				ls.b.Emit(&hir.GetElementPtr{
					Type:    structType,
					Base:    base,
					Indices: []hir.Value{hir.ConstInt{Text: "0"}, hir.ConstInt{Text: fmt.Sprintf("%d", idx)}},
					Dst:     elemPtr,
				})

				// Load boxed pointer
				boxed := ls.b.FreshTemp("tuple_elem_boxed")
				ls.b.Emit(&hir.Load{Type: "ptr", Src: elemPtr, Dst: boxed})

				// Unbox: load actual value from boxed pointer
				val := ls.b.FreshTemp("tuple_elem_val")
				ls.b.Emit(&hir.Load{Type: llvmElemType, Src: boxed, Dst: val})

				return val
			}
		} else if g, ok := baseType.(*types.Generic); ok {
			if s, ok := g.Base.(*types.Struct); ok {
				// Structs use type erasure - keep original field types
				// (constructor stores ptr, field access uses unboxing)
				fields = s.Fields
			} else if c, ok := g.Base.(*types.Class); ok {
				// Generic classes use monomorphization - substitute concrete types
				fields = make([]types.Field, len(c.Fields))
				subst := make(map[string]types.T)
				for i, tp := range c.TypeParams {
					if i < len(g.Args) {
						subst[tp.Name] = g.Args[i]
					}
				}
				for i, f := range c.Fields {
					fields[i] = types.Field{
						Name: f.Name,
						Type: substituteType(f.Type, subst),
					}
				}
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
					// Unbox by loading from the pointer
					unboxed := ls.b.FreshTemp("unboxed")
					ls.b.Emit(&hir.Load{
						Src:  dst,
						Dst:  unboxed,
						Type: targetType,
					})
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

		// Determine type tag and to_str function
		var typeTag hir.Value = hir.ConstInt{Text: "0", Type: "i32"}
		var toStrFunc hir.Value = hir.ConstStr{Text: "null"}

		if ls.info != nil {
			if t, ok := ls.info.Types[x].(*types.List); ok {
				typeTag = getTypeTag(t.Elem)
				toStrFunc = resolveToStrFunc(t.Elem)
			}
		}

		ls.b.Emit(&hir.Call{Dst: res, Fn: "list_new", Args: []hir.Value{typeTag, toStrFunc}})

		// Append elements
		for _, e := range x.Elems {
			val := ls.lowerExpr(e)

			// Cast to ptr for generic storage (void*)
			// This handles both pointers (bitcast) and integers (inttoptr)
			valPtr := ls.b.FreshTemp("val_ptr")
			ls.b.Emit(&hir.Cast{Dst: valPtr, Src: val, Type: "ptr"})

			ls.b.Emit(&hir.Call{Fn: "list_append", Args: []hir.Value{res, valPtr, typeTag}})
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
		// Check for decimal arithmetic first
		if ls.info != nil {
			lhsType := ls.info.Types[x.Lhs]
			if types.Equal(lhsType, types.Decimal) {
				lhs := ls.lowerExpr(x.Lhs)
				rhs := ls.lowerExpr(x.Rhs)
				dst := ls.b.FreshTemp("decimal_result")

				var fn string
				switch x.Op {
				case "+":
					fn = "__decimal_add"
				case "-":
					fn = "__decimal_sub"
				case "*":
					fn = "__decimal_mul"
				case "/":
					fn = "__decimal_div"
				case "==", "!=", "<", "<=", ">", ">=":
					// Comparison - use __decimal_cmp
					cmpResult := ls.b.FreshTemp("decimal_cmp")
					ls.b.Emit(&hir.Call{Dst: cmpResult, Fn: "__decimal_cmp", Args: []hir.Value{lhs, rhs}, Type: "i32"})

					// Convert cmp result (-1, 0, 1) to boolean
					var cmpOp string
					var cmpVal string
					switch x.Op {
					case "==":
						cmpOp = "=="
						cmpVal = "0"
					case "!=":
						cmpOp = "!="
						cmpVal = "0"
					case "<":
						cmpOp = "<"
						cmpVal = "0"
					case "<=":
						cmpOp = "<="
						cmpVal = "0"
					case ">":
						cmpOp = ">"
						cmpVal = "0"
					case ">=":
						cmpOp = ">="
						cmpVal = "0"
					}
					ls.b.Emit(&hir.BinaryOp{Op: cmpOp, LHS: cmpResult, RHS: hir.ConstInt{Text: cmpVal}, Dst: dst, Type: "i1"})
					return dst
				default:
					// Unsupported operator - fall through to default handling
					goto defaultBinaryOp
				}

				ls.b.Emit(&hir.Call{Dst: dst, Fn: fn, Args: []hir.Value{lhs, rhs}, Type: "ptr"})
				return dst
			}
		}

	defaultBinaryOp:
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
					case "&":
						dunder = "__and__"
					case "|":
						dunder = "__or__"
					case "^":
						dunder = "__xor__"
					case "<<":
						dunder = "__lshift__"
					case ">>":
						dunder = "__rshift__"
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

		// Check for operator overloading
		if ls.info != nil {
			cand := ls.info.BinOpOverloads[x]
			if cand != nil {
				// Dispatch to dunder method
				// We need to find the class that defined this method to get the correct mangled name
				lhsT := ls.info.Types[x.Lhs]
				if cls, ok := lhsT.(*types.Class); ok {
					// Map operator to dunder name
					var method string
					switch x.Op {
					case "+":
						method = "__add__"
					case "-":
						method = "__sub__"
					case "*":
						method = "__mul__"
					case "/":
						method = "__div__"
					case "%":
						method = "__mod__"
					case "**":
						method = "__pow__"
					case "|":
						method = "__or__"
					case "&":
						method = "__and__"
					case "^":
						method = "__xor__"
					case "<<":
						method = "__lshift__"
					case ">>":
						method = "__rshift__"
					case "==":
						method = "__eq__"
					case "!=":
						method = "__ne__" // Special handling: might be __eq__ negated
					case "<":
						method = "__lt__"
					case "<=":
						method = "__le__"
					case ">":
						method = "__gt__"
					case ">=":
						method = "__ge__"
					}

					// Handle != fallback to __eq__
					negateResult := false
					if x.Op == "!=" && method == "__ne__" {
						// Check if __ne__ actually exists in the candidate
						// The candidate type should match what we found in check
						// But check might have fallen back to __eq__
						// If the candidate matches __eq__, then we negate
						if eqMethod, ok := cls.Dunders["__eq__"]; ok && eqMethod == cand.Type {
							method = "__eq__"
							negateResult = true
						}
					}

					if method != "" {
						// Find defining class
						targetCls := cls
						for targetCls.Base != nil {
							if baseMethod, ok := targetCls.Base.Dunders[method]; ok && baseMethod == cand.Type {
								targetCls = targetCls.Base
							} else {
								break
							}
						}
						mangled := fmt.Sprintf("%s_%s", targetCls.Name, method)
						fmt.Fprintf(os.Stderr, "DEBUG lowerBinary: op=%s method=%s mangled=%s\n", x.Op, method, mangled)

						// Emit Call
						dst := ls.b.FreshTemp("binop_call")
						ls.b.Emit(&hir.Call{
							Dst:  dst,
							Fn:   mangled,
							Args: []hir.Value{lhs, rhs},
						})

						if negateResult {
							// Emit not
							notDst := ls.b.FreshTemp("ne_res")
							ls.b.Emit(&hir.BinaryOp{
								Op:   "==",
								LHS:  dst,
								RHS:  hir.ConstBool{Value: false},
								Dst:  notDst,
								Type: "i1",
							})
							return notDst
						}

						return dst
					}
				}
			}
		}

		// Handle tuple equality: compare elements, not pointers
		if (x.Op == "==" || x.Op == "!=") && ls.info != nil {
			if tupType, ok := ls.info.Types[x.Lhs].(*types.Tuple); ok {
				return ls.lowerTupleComparison(lhs, rhs, tupType, x.Op == "!=")
			}
		}

		// Handle tuple concatenation: t1 + t2 => combined tuple
		if x.Op == "+" && ls.info != nil {
			ltup, lok := ls.info.Types[x.Lhs].(*types.Tuple)
			rtup, rok := ls.info.Types[x.Rhs].(*types.Tuple)
			if lok && rok {
				return ls.lowerTupleConcatenation(lhs, rhs, ltup, rtup)
			}
		}

		// Handle tuple lexicographic comparison: <, >, <=, >=
		if (x.Op == "<" || x.Op == ">" || x.Op == "<=" || x.Op == ">=") && ls.info != nil {
			if tupType, ok := ls.info.Types[x.Lhs].(*types.Tuple); ok {
				return ls.lowerTupleLexicographic(lhs, rhs, tupType, x.Op)
			}
		}

		// Handle tuple membership: x in (a, b, c)
		if x.Op == "in" && ls.info != nil {
			// Get the tuple type - may be from Types map or from Idents (for variables)
			var rhsType types.T
			if t := ls.info.Types[x.Rhs]; t != nil {
				rhsType = t
			} else if id, ok := x.Rhs.(*ast.Ident); ok {
				if sym := ls.info.Idents[id]; sym != nil {
					rhsType = sym.Type
				}
			}
			if tupType, ok := rhsType.(*types.Tuple); ok {
				return ls.lowerTupleMembership(lhs, rhs, tupType)
			}

			// Handle list membership: x in list
			if _, ok := rhsType.(*types.List); ok {
				// Cast element to ptr for list_contains
				elemPtr := ls.b.FreshTemp("elem_ptr")
				ls.b.Emit(&hir.Cast{Dst: elemPtr, Src: lhs, Type: "ptr"})

				// Call list_contains -> returns i32
				foundI32 := ls.b.FreshTemp("found_i32")
				ls.b.Emit(&hir.Call{Dst: foundI32, Fn: "list_contains", Args: []hir.Value{rhs, elemPtr}, Type: "i32"})

				// Convert i32 to i1 (0 -> false, non-zero -> true)
				dst := ls.b.FreshTemp("found")
				ls.b.Emit(&hir.BinaryOp{Op: "!=", LHS: foundI32, RHS: hir.ConstInt{Text: "0"}, Dst: dst, Type: "i1"})
				return dst
			}

			// Handle set membership: x in set
			if _, ok := rhsType.(*types.Set); ok {
				// Cast element to ptr for set_contains
				elemPtr := ls.b.FreshTemp("elem_ptr")
				ls.b.Emit(&hir.Cast{Dst: elemPtr, Src: lhs, Type: "ptr"})

				// Call set_contains -> returns i32
				foundI32 := ls.b.FreshTemp("found_i32")
				ls.b.Emit(&hir.Call{Dst: foundI32, Fn: "set_contains", Args: []hir.Value{rhs, elemPtr}, Type: "i32"})

				// Convert i32 to i1
				dst := ls.b.FreshTemp("found")
				ls.b.Emit(&hir.BinaryOp{Op: "!=", LHS: foundI32, RHS: hir.ConstInt{Text: "0"}, Dst: dst, Type: "i1"})
				return dst
			}

			// Handle string membership: substr in str (substring check)
			if types.Equal(rhsType, types.Str) {
				// Call string_contains(haystack, needle) -> returns i32
				foundI32 := ls.b.FreshTemp("found_i32")
				ls.b.Emit(&hir.Call{Dst: foundI32, Fn: "string_contains", Args: []hir.Value{rhs, lhs}, Type: "i32"})

				// Convert i32 to i1
				dst := ls.b.FreshTemp("found")
				ls.b.Emit(&hir.BinaryOp{Op: "!=", LHS: foundI32, RHS: hir.ConstInt{Text: "0"}, Dst: dst, Type: "i1"})
				return dst
			}

			// Handle custom class with __contains__ dunder: x in obj calls obj.__contains__(x)
			if cls, ok := rhsType.(*types.Class); ok {
				if _, found := cls.Dunders["__contains__"]; found {
					// Call ClassName___contains__(obj, element) -> returns bool (i1)
					mangledName := fmt.Sprintf("%s___contains__", cls.Name)
					dst := ls.b.FreshTemp("contains")
					ls.b.Emit(&hir.Call{Dst: dst, Fn: mangledName, Args: []hir.Value{rhs, lhs}, Type: "i1"})
					return dst
				}
			}
		}

		// Determine result type based on operation
		var resultType string
		if x.Op == "==" || x.Op == "!=" || x.Op == "<" || x.Op == ">" || x.Op == "<=" || x.Op == ">=" || x.Op == "and" || x.Op == "or" || x.Op == "in" {
			// Comparison, logical, and membership operations return boolean (i1)
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

// lowerTupleComparison generates element-wise comparison for tuples.
// For (a, b) == (c, d), generates: a == c && b == d
// If negate is true, returns the negation for != operator.
func (ls *lowerState) lowerTupleComparison(lhs, rhs hir.Value, tupType *types.Tuple, negate bool) hir.Value {
	numElems := len(tupType.Elems)
	if numElems == 0 {
		// Empty tuples are always equal
		if negate {
			return hir.ConstBool{Value: false}
		}
		return hir.ConstBool{Value: true}
	}

	// Build struct type for GEP (all elements are ptr due to boxing)
	var elemTypes []string
	for range tupType.Elems {
		elemTypes = append(elemTypes, "ptr")
	}
	structType := "{" + strings.Join(elemTypes, ", ") + "}"

	// Start with true, AND each element comparison
	result := hir.ConstBool{Value: true}
	var lastResult hir.Value = result

	for i, elemType := range tupType.Elems {
		// Get element pointer from LHS tuple
		lhsElemPtr := ls.b.FreshTemp(fmt.Sprintf("lhs_elem%d_ptr", i))
		ls.b.Emit(&hir.GetElementPtr{
			Type:    structType,
			Base:    lhs,
			Indices: []hir.Value{hir.ConstInt{Text: "0"}, hir.ConstInt{Text: fmt.Sprintf("%d", i)}},
			Dst:     lhsElemPtr,
		})

		// Load boxed pointer
		lhsBoxed := ls.b.FreshTemp(fmt.Sprintf("lhs_elem%d_boxed", i))
		ls.b.Emit(&hir.Load{Type: "ptr", Src: lhsElemPtr, Dst: lhsBoxed})

		// Unbox: load actual value from boxed pointer
		llvmType := lowerType(elemType)
		lhsVal := ls.b.FreshTemp(fmt.Sprintf("lhs_elem%d", i))
		ls.b.Emit(&hir.Load{Type: llvmType, Src: lhsBoxed, Dst: lhsVal})

		// Get element from RHS tuple
		rhsElemPtr := ls.b.FreshTemp(fmt.Sprintf("rhs_elem%d_ptr", i))
		ls.b.Emit(&hir.GetElementPtr{
			Type:    structType,
			Base:    rhs,
			Indices: []hir.Value{hir.ConstInt{Text: "0"}, hir.ConstInt{Text: fmt.Sprintf("%d", i)}},
			Dst:     rhsElemPtr,
		})

		rhsBoxed := ls.b.FreshTemp(fmt.Sprintf("rhs_elem%d_boxed", i))
		ls.b.Emit(&hir.Load{Type: "ptr", Src: rhsElemPtr, Dst: rhsBoxed})

		rhsVal := ls.b.FreshTemp(fmt.Sprintf("rhs_elem%d", i))
		ls.b.Emit(&hir.Load{Type: llvmType, Src: rhsBoxed, Dst: rhsVal})

		// Compare elements
		cmp := ls.b.FreshTemp(fmt.Sprintf("cmp%d", i))
		ls.b.Emit(&hir.BinaryOp{
			Op:   "==",
			LHS:  lhsVal,
			RHS:  rhsVal,
			Dst:  cmp,
			Type: "i1",
		})

		// AND with previous result
		if i == 0 {
			lastResult = cmp
		} else {
			andResult := ls.b.FreshTemp(fmt.Sprintf("and%d", i))
			ls.b.Emit(&hir.BinaryOp{
				Op:   "and",
				LHS:  lastResult,
				RHS:  cmp,
				Dst:  andResult,
				Type: "i1",
			})
			lastResult = andResult
		}
	}

	// Negate if != operator
	if negate {
		negResult := ls.b.FreshTemp("tuple_neq")
		ls.b.Emit(&hir.BinaryOp{
			Op:   "==",
			LHS:  lastResult,
			RHS:  hir.ConstBool{Value: false},
			Dst:  negResult,
			Type: "i1",
		})
		return negResult
	}

	return lastResult
}

// lowerTupleConcatenation generates code for tuple1 + tuple2.
// Extracts all elements from both tuples and builds a new combined tuple.
func (ls *lowerState) lowerTupleConcatenation(lhs, rhs hir.Value, ltup, rtup *types.Tuple) hir.Value {
	llen := len(ltup.Elems)
	rlen := len(rtup.Elems)
	totalLen := llen + rlen

	if totalLen == 0 {
		// Empty tuple concatenation - allocate empty struct
		dst := ls.b.FreshTemp("concat_tuple")
		ls.b.Emit(&hir.Alloca{
			Dst:  dst,
			Type: "{}",
		})
		return dst
	}

	// Build struct type for result (all elements are ptr due to boxing)
	var resultElemTypes []string
	for i := 0; i < totalLen; i++ {
		resultElemTypes = append(resultElemTypes, "ptr")
	}
	resultStructType := "{" + strings.Join(resultElemTypes, ", ") + "}"

	// Allocate result tuple
	dst := ls.b.FreshTemp("concat_tuple")
	ls.b.Emit(&hir.Alloca{
		Dst:  dst,
		Type: resultStructType,
	})

	// Build struct types for source tuples
	var lElemTypes, rElemTypes []string
	for range ltup.Elems {
		lElemTypes = append(lElemTypes, "ptr")
	}
	for range rtup.Elems {
		rElemTypes = append(rElemTypes, "ptr")
	}
	lStructType := "{" + strings.Join(lElemTypes, ", ") + "}"
	rStructType := "{" + strings.Join(rElemTypes, ", ") + "}"

	// Copy elements from left tuple
	for i := 0; i < llen; i++ {
		// Get element pointer from source
		srcElemPtr := ls.b.FreshTemp(fmt.Sprintf("lhs_elem%d_ptr", i))
		ls.b.Emit(&hir.GetElementPtr{
			Type:    lStructType,
			Base:    lhs,
			Indices: []hir.Value{hir.ConstInt{Text: "0"}, hir.ConstInt{Text: fmt.Sprintf("%d", i)}},
			Dst:     srcElemPtr,
		})

		// Load boxed pointer
		boxedPtr := ls.b.FreshTemp(fmt.Sprintf("lhs_elem%d_boxed", i))
		ls.b.Emit(&hir.Load{Type: "ptr", Src: srcElemPtr, Dst: boxedPtr})

		// Get destination element pointer
		dstElemPtr := ls.b.FreshTemp(fmt.Sprintf("concat_elem%d_ptr", i))
		ls.b.Emit(&hir.GetElementPtr{
			Type:    resultStructType,
			Base:    dst,
			Indices: []hir.Value{hir.ConstInt{Text: "0"}, hir.ConstInt{Text: fmt.Sprintf("%d", i)}},
			Dst:     dstElemPtr,
		})

		// Store boxed pointer into result
		ls.b.Emit(&hir.Store{Dst: dstElemPtr, Val: boxedPtr})
	}

	// Copy elements from right tuple
	for i := 0; i < rlen; i++ {
		// Get element pointer from source
		srcElemPtr := ls.b.FreshTemp(fmt.Sprintf("rhs_elem%d_ptr", i))
		ls.b.Emit(&hir.GetElementPtr{
			Type:    rStructType,
			Base:    rhs,
			Indices: []hir.Value{hir.ConstInt{Text: "0"}, hir.ConstInt{Text: fmt.Sprintf("%d", i)}},
			Dst:     srcElemPtr,
		})

		// Load boxed pointer
		boxedPtr := ls.b.FreshTemp(fmt.Sprintf("rhs_elem%d_boxed", i))
		ls.b.Emit(&hir.Load{Type: "ptr", Src: srcElemPtr, Dst: boxedPtr})

		// Get destination element pointer (offset by llen)
		dstIdx := llen + i
		dstElemPtr := ls.b.FreshTemp(fmt.Sprintf("concat_elem%d_ptr", dstIdx))
		ls.b.Emit(&hir.GetElementPtr{
			Type:    resultStructType,
			Base:    dst,
			Indices: []hir.Value{hir.ConstInt{Text: "0"}, hir.ConstInt{Text: fmt.Sprintf("%d", dstIdx)}},
			Dst:     dstElemPtr,
		})

		// Store boxed pointer into result
		ls.b.Emit(&hir.Store{Dst: dstElemPtr, Val: boxedPtr})
	}

	return dst
}

// lowerTupleLexicographic generates element-wise lexicographic comparison for tuples.
// For (a, b) < (c, d), generates: (a < c) || (a == c && b < d)
func (ls *lowerState) lowerTupleLexicographic(lhs, rhs hir.Value, tupType *types.Tuple, op string) hir.Value {
	numElems := len(tupType.Elems)
	if numElems == 0 {
		// Empty tuples: < and > are false, <= and >= are true (equal tuples)
		if op == "<" || op == ">" {
			return hir.ConstBool{Value: false}
		}
		return hir.ConstBool{Value: true}
	}

	// Build struct type for GEP (all elements are ptr due to boxing)
	var elemTypes []string
	for range tupType.Elems {
		elemTypes = append(elemTypes, "ptr")
	}
	structType := "{" + strings.Join(elemTypes, ", ") + "}"

	// For lexicographic comparison, we compare element by element:
	// (a0, a1, ..., an) < (b0, b1, ..., bn) means:
	// a0 < b0 || (a0 == b0 && (a1 < b1 || (a1 == b1 && ...)))

	// We'll simplify by iterating and building the result
	// Start with result = false for strict (<, >), or check for equality for non-strict (<=, >=)
	elemType := tupType.Elems[0]
	llvmType := lowerType(elemType)

	// Determine the comparison operator to use
	var strictOp string
	switch op {
	case "<", "<=":
		strictOp = "<"
	case ">", ">=":
		strictOp = ">"
	}

	// For each element, extract and compare
	var lastResult hir.Value = hir.ConstBool{Value: false}

	for i := numElems - 1; i >= 0; i-- {
		// Get element from LHS
		lhsElemPtr := ls.b.FreshTemp(fmt.Sprintf("lex_lhs_elem%d_ptr", i))
		ls.b.Emit(&hir.GetElementPtr{
			Type:    structType,
			Base:    lhs,
			Indices: []hir.Value{hir.ConstInt{Text: "0"}, hir.ConstInt{Text: fmt.Sprintf("%d", i)}},
			Dst:     lhsElemPtr,
		})
		lhsBoxed := ls.b.FreshTemp(fmt.Sprintf("lex_lhs_elem%d_boxed", i))
		ls.b.Emit(&hir.Load{Type: "ptr", Src: lhsElemPtr, Dst: lhsBoxed})
		lhsVal := ls.b.FreshTemp(fmt.Sprintf("lex_lhs_elem%d", i))
		ls.b.Emit(&hir.Load{Type: llvmType, Src: lhsBoxed, Dst: lhsVal})

		// Get element from RHS
		rhsElemPtr := ls.b.FreshTemp(fmt.Sprintf("lex_rhs_elem%d_ptr", i))
		ls.b.Emit(&hir.GetElementPtr{
			Type:    structType,
			Base:    rhs,
			Indices: []hir.Value{hir.ConstInt{Text: "0"}, hir.ConstInt{Text: fmt.Sprintf("%d", i)}},
			Dst:     rhsElemPtr,
		})
		rhsBoxed := ls.b.FreshTemp(fmt.Sprintf("lex_rhs_elem%d_boxed", i))
		ls.b.Emit(&hir.Load{Type: "ptr", Src: rhsElemPtr, Dst: rhsBoxed})
		rhsVal := ls.b.FreshTemp(fmt.Sprintf("lex_rhs_elem%d", i))
		ls.b.Emit(&hir.Load{Type: llvmType, Src: rhsBoxed, Dst: rhsVal})

		if i == numElems-1 {
			// Last element: just compare with the operator
			cmp := ls.b.FreshTemp(fmt.Sprintf("lex_cmp%d", i))
			ls.b.Emit(&hir.BinaryOp{
				Op:   op, // Use the original operator for the last element
				LHS:  lhsVal,
				RHS:  rhsVal,
				Dst:  cmp,
				Type: "i1",
			})
			lastResult = cmp
		} else {
			// Not the last element: check (lhs[i] < rhs[i]) || (lhs[i] == rhs[i] && lastResult)
			// First, strict comparison
			strictCmp := ls.b.FreshTemp(fmt.Sprintf("lex_strict%d", i))
			ls.b.Emit(&hir.BinaryOp{
				Op:   strictOp,
				LHS:  lhsVal,
				RHS:  rhsVal,
				Dst:  strictCmp,
				Type: "i1",
			})

			// Equality comparison
			eqCmp := ls.b.FreshTemp(fmt.Sprintf("lex_eq%d", i))
			ls.b.Emit(&hir.BinaryOp{
				Op:   "==",
				LHS:  lhsVal,
				RHS:  rhsVal,
				Dst:  eqCmp,
				Type: "i1",
			})

			// AND: eq && lastResult
			andResult := ls.b.FreshTemp(fmt.Sprintf("lex_and%d", i))
			ls.b.Emit(&hir.BinaryOp{
				Op:   "and",
				LHS:  eqCmp,
				RHS:  lastResult,
				Dst:  andResult,
				Type: "i1",
			})

			// OR: strict || (eq && lastResult)
			orResult := ls.b.FreshTemp(fmt.Sprintf("lex_or%d", i))
			ls.b.Emit(&hir.BinaryOp{
				Op:   "or",
				LHS:  strictCmp,
				RHS:  andResult,
				Dst:  orResult,
				Type: "i1",
			})

			lastResult = orResult
		}
	}

	return lastResult
}

// lowerTupleMembership generates element-wise membership check for tuples.
// For x in (a, b, c), generates: (x == a) || (x == b) || (x == c)
func (ls *lowerState) lowerTupleMembership(needle, tup hir.Value, tupType *types.Tuple) hir.Value {
	numElems := len(tupType.Elems)
	if numElems == 0 {
		// Empty tuple: nothing can be in it
		return hir.ConstBool{Value: false}
	}

	// Build struct type for GEP (all elements are ptr due to boxing)
	var elemTypes []string
	for range tupType.Elems {
		elemTypes = append(elemTypes, "ptr")
	}
	structType := "{" + strings.Join(elemTypes, ", ") + "}"

	// Get element type (all elements have same type for homogeneous tuple)
	elemType := tupType.Elems[0]
	llvmType := lowerType(elemType)

	// Unbox the needle if it's boxed
	needleUnboxed := needle
	// For now assume needle is already the right type

	// For each element, compare with needle and OR results together
	var result hir.Value = hir.ConstBool{Value: false}

	for i := 0; i < numElems; i++ {
		// Get element from tuple
		elemPtr := ls.b.FreshTemp(fmt.Sprintf("member_elem%d_ptr", i))
		ls.b.Emit(&hir.GetElementPtr{
			Type:    structType,
			Base:    tup,
			Indices: []hir.Value{hir.ConstInt{Text: "0"}, hir.ConstInt{Text: fmt.Sprintf("%d", i)}},
			Dst:     elemPtr,
		})
		elemBoxed := ls.b.FreshTemp(fmt.Sprintf("member_elem%d_boxed", i))
		ls.b.Emit(&hir.Load{Type: "ptr", Src: elemPtr, Dst: elemBoxed})

		// For ptr types (strings), the box value IS the string pointer
		// For value types (int, float), we need to load the value from the box
		var elemVal hir.Value
		if llvmType == "ptr" {
			elemVal = elemBoxed // String is already the ptr we need
		} else {
			val := ls.b.FreshTemp(fmt.Sprintf("member_elem%d", i))
			ls.b.Emit(&hir.Load{Type: llvmType, Src: elemBoxed, Dst: val})
			elemVal = val
		}

		// Compare needle with element - use string_eq for strings
		cmp := ls.b.FreshTemp(fmt.Sprintf("member_cmp%d", i))
		if elemType == types.Str {
			// String comparison needs strcmp - returns 0 if equal
			strcmpResult := ls.b.FreshTemp(fmt.Sprintf("member_strcmp%d", i))
			ls.b.Emit(&hir.Call{
				Fn:   "strcmp",
				Args: []hir.Value{needleUnboxed, elemVal},
				Dst:  strcmpResult,
				Type: "i32",
			})
			// strcmp returns 0 for equal, so compare with 0
			ls.b.Emit(&hir.BinaryOp{
				Op:   "==",
				LHS:  strcmpResult,
				RHS:  hir.ConstInt{Text: "0", Type: "i32"},
				Dst:  cmp,
				Type: "i1",
			})
		} else {
			ls.b.Emit(&hir.BinaryOp{
				Op:   "==",
				LHS:  needleUnboxed,
				RHS:  elemVal,
				Dst:  cmp,
				Type: "i1",
			})
		}

		// OR with previous result
		if i == 0 {
			result = cmp
		} else {
			orResult := ls.b.FreshTemp(fmt.Sprintf("member_or%d", i))
			ls.b.Emit(&hir.BinaryOp{
				Op:   "or",
				LHS:  result,
				RHS:  cmp,
				Dst:  orResult,
				Type: "i1",
			})
			result = orResult
		}
	}

	return result
}

// lowerListComp builds HIR for list comprehensions by expanding them to loops:
//
//	let %result = list_new(...)
//	let %idx = 0
//	while %idx < len(%iter):
//	    let %elem = list_get(%iter, %idx)
//	    if <filter_cond>:  // optional
//	        list_append(%result, <transformed_elem>)
//	    %idx = %idx + 1
//
// Returns %result as the value of the comprehension.
func (ls *lowerState) lowerListComp(c *ast.ListComp) hir.Value {
	if len(c.Clauses) == 0 {
		// Degenerate: no iteration clauses, just return empty list
		res := ls.b.FreshTemp("list")
		ls.b.Emit(&hir.Call{Dst: res, Fn: "list_new", Args: []hir.Value{
			hir.ConstInt{Text: "0", Type: "i32"},
			hir.ConstStr{Text: "null"},
		}})
		return res
	}

	// For now, handle single clause comprehensions: [expr for x in iter (if cond)?]
	clause := c.Clauses[0]

	// 1. Create result list
	res := ls.b.FreshTemp("comp_result")
	var typeTag hir.Value = hir.ConstInt{Text: "0", Type: "i32"}
	var toStrFunc hir.Value = hir.ConstStr{Text: "null"}

	// Try to get element type for proper type tag
	if ls.info != nil {
		if t, ok := ls.info.Types[c].(*types.List); ok {
			typeTag = getTypeTag(t.Elem)
			toStrFunc = resolveToStrFunc(t.Elem)
		}
	}
	ls.b.Emit(&hir.Call{Dst: res, Fn: "list_new", Args: []hir.Value{typeTag, toStrFunc}})

	// 2. Detect range(start, stop) call for efficient lowering
	// Check BEFORE lowering the iterable to avoid emitting a range() call
	isRange := false
	var rangeStart, rangeStop hir.Value
	var iterVal hir.Value

	if call, ok := clause.Iter.(*ast.CallExpr); ok {
		if id, ok := call.Callee.(*ast.Ident); ok && id.Name == "range" {
			isRange = true
			if len(call.Args) == 1 {
				// range(stop): 0 to stop-1
				rangeStart = hir.ConstInt{Text: "0", Type: "i64"}
				rangeStop = ls.lowerExpr(call.Args[0])
				// Cast to i64 if needed
				stopI64 := ls.b.FreshTemp("stop_i64")
				ls.b.Emit(&hir.Cast{Dst: stopI64, Src: rangeStop, Type: "i64"})
				rangeStop = stopI64
			} else if len(call.Args) >= 2 {
				// range(start, stop)
				rangeStart = ls.lowerExpr(call.Args[0])
				startI64 := ls.b.FreshTemp("start_i64")
				ls.b.Emit(&hir.Cast{Dst: startI64, Src: rangeStart, Type: "i64"})
				rangeStart = startI64

				rangeStop = ls.lowerExpr(call.Args[1])
				stopI64 := ls.b.FreshTemp("stop_i64")
				ls.b.Emit(&hir.Cast{Dst: stopI64, Src: rangeStop, Type: "i64"})
				rangeStop = stopI64
			}
		}
	}

	// Only lower the iterable if it's not a range (range is handled inline)
	if !isRange {
		iterVal = ls.lowerExpr(clause.Iter)
	}

	// 3. Get iteration bounds
	var lenTemp hir.Value
	if isRange {
		lenTemp = rangeStop
	} else {
		lenTemp = ls.b.FreshTemp("comp_len")
		ls.b.Emit(&hir.Call{Dst: lenTemp.(hir.Temp), Fn: "list_len", Args: []hir.Value{iterVal}, Type: "i64"})
	}

	// 4. Allocate index variable
	idxPtr := ls.b.FreshTemp("comp_idx_ptr")
	ls.b.Emit(&hir.Alloca{Dst: idxPtr, Type: "i64", Count: 1})
	if isRange {
		ls.b.Emit(&hir.Store{Dst: idxPtr, Val: rangeStart})
	} else {
		ls.b.Emit(&hir.Store{Dst: idxPtr, Val: hir.ConstInt{Text: "0", Type: "i64"}})
	}

	// 5. Create condition block
	condBlk := ls.b.NewBlock("comp_cond")
	oldCur := ls.b.Block()

	ls.b.SetBlock(condBlk)
	idxVal := ls.b.FreshTemp("comp_idx")
	ls.b.Emit(&hir.Load{Type: "i64", Src: idxPtr, Dst: idxVal})
	condTemp := ls.b.FreshTemp("comp_cond")
	ls.b.Emit(&hir.BinaryOp{Dst: condTemp, Op: "<", LHS: idxVal, RHS: lenTemp, Type: "i1"})
	ls.b.SetBlock(oldCur)

	// 6. Create body block
	bodyBlk := ls.b.NewBlock("comp_body")
	ls.b.SetBlock(bodyBlk)

	// Load current index for use in body
	idxBody := ls.b.FreshTemp("comp_idx_body")
	ls.b.Emit(&hir.Load{Type: "i64", Src: idxPtr, Dst: idxBody})

	// Get loop variable name from Target
	var loopVarName string
	if target, ok := clause.Target.(*ast.Ident); ok {
		loopVarName = target.Name
	} else {
		loopVarName = "__x" // fallback
	}

	// Get element value and bind loop variable
	var elemVal hir.Value
	if isRange {
		// For range, the index IS the element value (cast to i32)
		elemI32 := ls.b.FreshTemp("range_elem")
		ls.b.Emit(&hir.Cast{Dst: elemI32, Src: idxBody, Type: "i32"})
		elemVal = elemI32
	} else {
		// Cast to i32 for list_get
		idxI32 := ls.b.FreshTemp("comp_idx_i32")
		ls.b.Emit(&hir.Cast{Dst: idxI32, Src: idxBody, Type: "i32"})

		// Get element: list_get(iter, idx)
		elemPtr := ls.b.FreshTemp("comp_elem_ptr")
		ls.b.Emit(&hir.Call{Dst: elemPtr, Fn: "list_get", Args: []hir.Value{iterVal, idxI32}, Type: "ptr"})

		// Cast to element type (default i32 for int lists)
		elemTemp := ls.b.FreshTemp("comp_elem")
		ls.b.Emit(&hir.Cast{Dst: elemTemp, Src: elemPtr, Type: "i32"})
		elemVal = elemTemp
	}

	// Bind loop variable for use in element expression
	ls.b.Emit(&hir.Let{Name: loopVarName, Init: elemVal, Type: types.Int})

	// 7. Evaluate element expression (the transformed value)
	transformedElem := ls.lowerExpr(c.Elem)
	if transformedElem == nil {
		transformedElem = hir.ConstInt{Text: "0"}
	}

	// 8. Handle filter condition if present
	if clause.If != nil {
		// Evaluate filter condition
		filterCond := ls.lowerExpr(clause.If)

		// Create then block for appending
		thenBlk := ls.b.NewBlock("comp_then")
		ls.b.SetBlock(thenBlk)

		// Cast to ptr for list_append
		valPtr := ls.b.FreshTemp("val_ptr")
		ls.b.Emit(&hir.Cast{Dst: valPtr, Src: transformedElem, Type: "ptr"})
		ls.b.Emit(&hir.Call{Fn: "list_append", Args: []hir.Value{res, valPtr, typeTag}})

		// Switch back to body and emit conditional
		ls.b.SetBlock(bodyBlk)
		ls.b.Emit(&hir.If{Cond: filterCond, Then: thenBlk, Else: nil})
	} else {
		// No filter: always append
		valPtr := ls.b.FreshTemp("val_ptr")
		ls.b.Emit(&hir.Cast{Dst: valPtr, Src: transformedElem, Type: "ptr"})
		ls.b.Emit(&hir.Call{Fn: "list_append", Args: []hir.Value{res, valPtr, typeTag}})
	}

	// 9. Increment index
	incTemp := ls.b.FreshTemp("comp_inc")
	ls.b.Emit(&hir.BinaryOp{Dst: incTemp, Op: "+", LHS: idxBody, RHS: hir.ConstInt{Text: "1", Type: "i64"}, Type: "i64"})
	ls.b.Emit(&hir.Store{Dst: idxPtr, Val: incTemp})

	// 10. Back to original block and emit while loop
	ls.b.SetBlock(oldCur)
	ls.b.Emit(&hir.While{Cond: condTemp, CondBlock: condBlk, Body: bodyBlk})

	return res
}

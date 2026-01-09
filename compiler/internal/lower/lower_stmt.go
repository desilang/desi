package lower

import (
	"fmt"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/types"
)

// lowerBlock lowers a block of statements
func (ls *lowerState) lowerBlock(blk *ast.Block) {
	for _, s := range blk.Stmts {
		if ls.terminated {
			break
		}
		ls.lowerStmt(s)
	}
	// End-of-root-block finalization (only for outermost scope).
	if len(ls.scopes) == 1 && !ls.terminated {
		ls.emitScopeDrops(ls.cur())
		// If no explicit return ran, emit a default one (Tier-0: i32 0).
		ls.b.Emit(&hir.Ret{Val: nil})
		ls.terminated = true
	}
}

// lowerStmt lowers a single statement
func (ls *lowerState) lowerStmt(s ast.Stmt) {
	switch s := s.(type) {
	case *ast.LetStmt:
		// Handle tuple destructuring: let (a, b, c) = tuple or let (first, *rest) = tuple
		if len(s.Pattern) > 0 {
			// Evaluate the RHS tuple expression
			tupleVal := ls.lowerExpr(s.Value)

			// Get tuple type from type checker
			tupleType, _ := ls.info.Types[s.Value].(*types.Tuple)
			if tupleType == nil {
				// Fallback: shouldn't happen if type checker did its job
				return
			}

			// Build struct type for GEP (all elements are ptr due to boxing)
			var elemTypes []string
			for range tupleType.Elems {
				elemTypes = append(elemTypes, "ptr")
			}
			structType := "{" + strings.Join(elemTypes, ", ") + "}"

			hasRest := s.RestIndex >= 0

			// Extract each element and bind to pattern variable
			for i, ident := range s.Pattern {
				if ident.Name == "_" {
					// Ignore pattern - don't bind
					continue
				}

				if hasRest && i == s.RestIndex {
					// This is the rest pattern - create a new tuple with remaining elements
					// Calculate which tuple elements go to *rest
					beforeRest := s.RestIndex
					afterRest := len(s.Pattern) - (s.RestIndex + 1)
					restStart := beforeRest
					restEnd := len(tupleType.Elems) - afterRest
					restCount := restEnd - restStart

					if restCount == 0 {
						// Empty rest tuple - allocate empty struct
						dst := ls.b.FreshTemp("rest_tuple")
						ls.b.Emit(&hir.Call{Dst: dst, Fn: "malloc", Args: []hir.Value{hir.ConstInt{Text: "8", Type: "i64"}}, Type: "ptr"})

						ls.cur().locals = append(ls.cur().locals, ident.Name)
						ls.b.Emit(&hir.Let{Name: ident.Name, Init: dst, Type: types.TupleOf()})
						ls.cur().types[ident.Name] = types.TupleOf()
					} else {
						// Allocate new tuple for rest elements
						restSize := restCount * 8
						restTuple := ls.b.FreshTemp("rest_tuple")
						ls.b.Emit(&hir.Call{Dst: restTuple, Fn: "malloc", Args: []hir.Value{hir.ConstInt{Text: fmt.Sprintf("%d", restSize), Type: "i64"}}, Type: "ptr"})

						// Build rest tuple struct type
						var restElemTypes []string
						for j := 0; j < restCount; j++ {
							restElemTypes = append(restElemTypes, "ptr")
						}
						restStructType := "{" + strings.Join(restElemTypes, ", ") + "}"

						// Copy elements from source tuple to rest tuple
						for j := 0; j < restCount; j++ {
							srcIdx := restStart + j

							// Get boxed ptr from source tuple
							srcPtr := ls.b.FreshTemp("rest_src_ptr")
							ls.b.Emit(&hir.GetElementPtr{
								Type:    structType,
								Base:    tupleVal,
								Indices: []hir.Value{hir.ConstInt{Text: "0"}, hir.ConstInt{Text: fmt.Sprintf("%d", srcIdx)}},
								Dst:     srcPtr,
							})
							srcBoxed := ls.b.FreshTemp("rest_src_boxed")
							ls.b.Emit(&hir.Load{Type: "ptr", Src: srcPtr, Dst: srcBoxed})

							// Store in rest tuple
							dstPtr := ls.b.FreshTemp("rest_dst_ptr")
							ls.b.Emit(&hir.GetElementPtr{
								Type:    restStructType,
								Base:    restTuple,
								Indices: []hir.Value{hir.ConstInt{Text: "0"}, hir.ConstInt{Text: fmt.Sprintf("%d", j)}},
								Dst:     dstPtr,
							})
							ls.b.Emit(&hir.Store{Dst: dstPtr, Val: srcBoxed})
						}

						// Get rest type from type checker
						restType := ls.info.Types[&s.Pattern[i]]
						if restType == nil {
							restType = types.TupleOf(tupleType.Elems[restStart:restEnd]...)
						}

						// Bind rest tuple to variable
						ls.cur().locals = append(ls.cur().locals, ident.Name)
						ls.b.Emit(&hir.Let{Name: ident.Name, Init: restTuple, Type: restType})
						ls.cur().types[ident.Name] = restType
					}
				} else {
					// Regular element - map to correct tuple index
					var tupleIdx int
					if hasRest && i > s.RestIndex {
						// After rest: count from end
						afterRestPos := len(s.Pattern) - i // position from end (1-based)
						tupleIdx = len(tupleType.Elems) - afterRestPos
					} else {
						// Before rest (or no rest): direct index
						tupleIdx = i
					}

					elemType := tupleType.Elems[tupleIdx]
					llvmElemType := lowerType(elemType)

					// Get pointer to element slot
					elemPtr := ls.b.FreshTemp("tup_elem_ptr")
					ls.b.Emit(&hir.GetElementPtr{
						Type:    structType,
						Base:    tupleVal,
						Indices: []hir.Value{hir.ConstInt{Text: "0"}, hir.ConstInt{Text: fmt.Sprintf("%d", tupleIdx)}},
						Dst:     elemPtr,
					})

					// Load boxed pointer
					boxed := ls.b.FreshTemp("tup_elem_boxed")
					ls.b.Emit(&hir.Load{Type: "ptr", Src: elemPtr, Dst: boxed})

					// Unbox: load actual value from boxed pointer
					// For ptr types (strings), the boxed ptr IS the value
					var val hir.Value
					if llvmElemType == "ptr" {
						val = boxed
					} else {
						unboxed := ls.b.FreshTemp("tup_elem_val")
						ls.b.Emit(&hir.Load{Type: llvmElemType, Src: boxed, Dst: unboxed})
						val = unboxed
					}

					// Bind to variable
					ls.cur().locals = append(ls.cur().locals, ident.Name)
					if s.Mutable {
						ls.cur().mutable[ident.Name] = true
						ls.b.Emit(&hir.Let{Name: ident.Name, Init: nil, Type: elemType})
						ls.b.Emit(&hir.Store{Dst: hir.Var{Name: ident.Name}, Val: val})
					} else {
						ls.b.Emit(&hir.Let{Name: ident.Name, Init: val, Type: elemType})
					}

					// Track type for drop
					ls.cur().types[ident.Name] = elemType
				}
			}
			return
		}

		// Single variable binding (original logic)
		// shadowing in same scope drops previous
		if ls.hasLocal(s.Name.Name) {
			ls.dropLocalByName(s.Name.Name)
			ls.removeLocal(s.Name.Name)
		}
		// record type shape and extract type from type checker
		var varType interface{}
		if ls.info != nil {
			if sym := ls.info.Idents[&s.Name]; sym != nil {
				varType = sym.Type
				if isRcLike(sym.Type) {
					ls.cur().rcLike[s.Name.Name] = true
				}
			}
			// Fallback: if sym is nil, get type from RHS expression (for nested class calls etc)
			if varType == nil && s.Value != nil {
				if t := ls.info.Types[s.Value]; t != nil {
					varType = t
				}
			}
		}
		ls.cur().locals = append(ls.cur().locals, s.Name.Name)

		// Store type for Drop instruction
		// IMPORTANT: Don't track variables initialized from index expressions for drop
		// Index expressions return borrowed references, not owned values
		skipDrop := false
		if s.Value != nil {
			if _, isIndexExpr := s.Value.(*ast.IndexExpr); isIndexExpr {
				skipDrop = true
			}
			// Also skip drop for MutexGuard.value field access - it's borrowed from the mutex
			if field, isFieldExpr := s.Value.(*ast.FieldExpr); isFieldExpr {
				if ls.info != nil {
					if baseType := ls.info.Types[field.X]; baseType != nil {
						if _, isMutexGuard := baseType.(*types.MutexGuard); isMutexGuard {
							skipDrop = true
							if ls.cur().borrowed == nil {
								ls.cur().borrowed = map[string]bool{}
							}
							ls.cur().borrowed[s.Name.Name] = true
						}
					}
				}
			}
			// Also skip drop for dict.get() - returns borrowed reference to value in dict
			if call, isCallExpr := s.Value.(*ast.CallExpr); isCallExpr {
				if field, ok := call.Callee.(*ast.FieldExpr); ok && field.Name.Name == "get" {
					if ls.info != nil {
						if baseType := ls.info.Types[field.X]; baseType != nil {
							if _, isDict := baseType.(*types.Dict); isDict {
								skipDrop = true
								if ls.cur().borrowed == nil {
									ls.cur().borrowed = map[string]bool{}
								}
								ls.cur().borrowed[s.Name.Name] = true
							}
						}
					}
				}
			}
		}

		if varType != nil && !skipDrop {
			if t, ok := varType.(types.T); ok {
				ls.cur().types[s.Name.Name] = t
			}
		}

		// Track if variable is mutable
		if s.Mutable {
			ls.cur().mutable[s.Name.Name] = true
		}

		var init hir.Value
		if s.Value != nil {
			// detect trivial move: let y = x
			if id, ok := s.Value.(*ast.Ident); ok {
				init = ls.lowerExpr(s.Value)
				if ls.hasLocal(id.Name) {
					ls.cur().moved[id.Name] = true
				}
			} else {
				init = ls.lowerExpr(s.Value)
			}

			// M15: Unbox result from generic functions if needed
			// If the call is to a generic function and the expected type is primitive, unbox
			if call, ok := s.Value.(*ast.CallExpr); ok {
				if id, ok := call.Callee.(*ast.Ident); ok && ls.info != nil {
					// Check if calling a generic function
					isGeneric := false
					if set, ok := ls.info.Funcs[id.Name]; ok && len(set.Cands) > 0 {
						if set.Cands[0].Decl != nil && len(set.Cands[0].Decl.TypeParams) > 0 {
							isGeneric = true
						}
					}

					if isGeneric {
						// Generic function returns ptr, but we might need primitive
						expectedType := varType
						if expectedType != nil {
							if t, ok := expectedType.(types.T); ok {
								expectedLowered := lowerType(t)
								if expectedLowered != "ptr" && expectedLowered != "void" {
									// Need to unbox: load from ptr
									unboxed := ls.b.FreshTemp("unboxed")
									ls.b.Emit(&hir.Load{
										Type: expectedLowered,
										Src:  init,
										Dst:  unboxed,
									})
									init = unboxed
								}
							}
						}
					}
				}
			}
		}

		// For mutable variables, always allocate storage
		if s.Mutable {
			// Emit Let without init (will be allocated by backend)
			ls.b.Emit(&hir.Let{Name: s.Name.Name, Init: nil, Type: varType})
			// If there's an initial value, store it
			if init != nil {
				ls.b.Emit(&hir.Store{
					Dst: hir.Var{Name: s.Name.Name},
					Val: init,
				})
			}
		} else {
			// Immutable: use SSA directly
			ls.b.Emit(&hir.Let{Name: s.Name.Name, Init: init, Type: varType})
		}

		// M7C: if init was a temp that came from ArenaAlloc, mark this local as arena-owned.
		if t, ok := init.(hir.Temp); ok {
			if ls.tempsFromArenaAlloc[t.Name] {
				ls.cur().arenaOwned[s.Name.Name] = true
			}
			// Consume temp so it's not dropped
			ls.consumeTemp(t)
		}

	case *ast.AssignStmt:
		if len(s.LHS) == 1 && len(s.RHS) == 1 {
			// Check for static field assignment (ClassName.FIELD = value)
			if fieldExpr, ok := s.LHS[0].(*ast.FieldExpr); ok {
				if ls.info != nil {
					if id, ok := fieldExpr.X.(*ast.Ident); ok {
						if sym := ls.info.Idents[id]; sym != nil {
							if cls, ok := sym.Type.(*types.Class); ok {
								// Check if it's a static field
								fieldName := fieldExpr.Name.Name
								curr := cls
								for curr != nil {
									if _, found := curr.StaticFields[fieldName]; found {
										// Emit Store to global: @ClassName_FieldName
										globalName := fmt.Sprintf("%s_%s", cls.Name, fieldName)
										rhs := ls.lowerExpr(s.RHS[0])
										ls.b.Emit(&hir.Store{
											Dst: hir.Var{Name: "@" + globalName},
											Val: rhs,
										})
										ls.consumeTemp(rhs)
										return
									}
									curr = curr.Base
								}
							}
						}
					}
				}
			}

			// Check for index assignment with __setitem__ (obj[idx] = value)
			if indexExpr, ok := s.LHS[0].(*ast.IndexExpr); ok {
				if ls.info != nil {
					objType := ls.info.Types[indexExpr.X]
					if cls, ok := objType.(*types.Class); ok {
						if _, found := cls.Dunders["__setitem__"]; found {
							// Desugar obj[idx] = value to obj.__setitem__(idx, value)
							obj := ls.lowerExpr(indexExpr.X)
							idx := ls.lowerExpr(indexExpr.Idx)
							val := ls.lowerExpr(s.RHS[0])

							// Call __setitem__ method
							mangledName := fmt.Sprintf("%s___setitem__", cls.Name)
							dst := ls.b.FreshTemp("setitem")
							ls.b.Emit(&hir.Call{
								Dst:  dst,
								Fn:   mangledName,
								Args: []hir.Value{obj, idx, val},
								Type: "i32", // __setitem__ typically returns none
							})
							ls.consumeTemp(val)
							return
						}
					}
				}
			}

			// Handle list index assignment: list[idx] = value
			if indexExpr, ok := s.LHS[0].(*ast.IndexExpr); ok {
				if ls.info != nil {
					objType := ls.info.Types[indexExpr.X]
					if _, ok := objType.(*types.List); ok {
						// Lower list, index, and value
						list := ls.lowerExpr(indexExpr.X)
						idx := ls.lowerExpr(indexExpr.Idx)
						val := ls.lowerExpr(s.RHS[0])

						// Sign-extend index to i64
						idx64 := ls.b.FreshTemp("idx_i64")
						ls.b.Emit(&hir.Cast{Dst: idx64, Src: idx, Type: "i64"})

						// Cast value to ptr for generic storage
						valPtr := ls.b.FreshTemp("val_ptr")
						ls.b.Emit(&hir.Cast{Dst: valPtr, Src: val, Type: "ptr"})

						// Call list_set
						ls.b.Emit(&hir.Call{Fn: "list_set", Args: []hir.Value{list, idx64, valPtr}})
						ls.consumeTemp(val)
						return
					}
				}
			}

			// Handle field assignment: obj.field = val
			if field, ok := s.LHS[0].(*ast.FieldExpr); ok {
				// Lower receiver
				recv := ls.lowerExpr(field.X)
				rhs := ls.lowerExpr(s.RHS[0])

				// Mark RHS variable as moved - field now owns it
				if id, ok := s.RHS[0].(*ast.Ident); ok {
					ls.cur().moved[id.Name] = true
				}

				// Get field info
				fieldName := field.Name.Name
				var recvType types.T
				if ls.info != nil {
					recvType = ls.info.Types[field.X]
				}

				// Calculate offset
				offset := 0
				found := false
				var fieldType types.T

				// Helper to find field in struct/class
				findField := func(fields []types.Field) {
					for _, f := range fields {
						if f.Name == fieldName {
							found = true
							fieldType = f.Type
							break
						}
						offset += getSize(f.Type)
					}
				}

				if recvType != nil {
					if cls, ok := recvType.(*types.Class); ok {
						findField(cls.Fields)
					} else if gen, ok := recvType.(*types.Generic); ok {
						// Handle generic class instances like Box<int>
						if cls, ok := gen.Base.(*types.Class); ok {
							findField(cls.Fields)
						}
					} else if st, ok := recvType.(*types.Struct); ok {
						findField(st.Fields)
					}
				}

				if found {
					// Emit GEP
					fieldPtr := ls.b.FreshTemp("field_ptr")
					ls.b.Emit(&hir.GetElementPtr{
						Type:    "i8", // We treat objects as i8* for offset calculation
						Base:    recv,
						Indices: []hir.Value{hir.ConstInt{Text: fmt.Sprintf("%d", offset)}},
						Dst:     fieldPtr,
					})

					// Box if necessary (primitive -> ptr)
					// Fields in classes are currently stored as ptr (boxed) if they are generic or if we want uniform layout?
					// Actually, getSize returns 8 for everything in Tier-0?
					// Let's check getSize.
					// If fields are typed, we might need to cast the pointer to the field type?
					// But GEP returns i8*. We should cast it to fieldType*.
					// Or just Store to i8* if we cast the value?

					// For Tier-0, we assume everything is 8 bytes (ptr or i64).
					// But primitives like i32 are 4 bytes.
					// If the field type is i32, we should store i32.

					// Cast fieldPtr to fieldType*
					_ = lowerType(fieldType) // Consumed for now to avoid unused var error if we need it later
					typedPtr := ls.b.FreshTemp("typed_field_ptr")
					ls.b.Emit(&hir.Cast{Dst: typedPtr, Src: fieldPtr, Type: "ptr"}) // Cast i8* to ptr (void*)?
					// Actually, Store takes Dst (ptr) and Val.
					// LLVM Store: store <ty> <val>, <ty>* <ptr>
					// We need to make sure Val has the correct type.

					// If fieldLowerType is "ptr", we just store.
					// If fieldLowerType is "i32", we store i32.

					ls.b.Emit(&hir.Store{
						Dst: typedPtr,
						Val: rhs,
					})
				} else {
					// Fallback for unknown fields (shouldn't happen if type checked)
					// Just ignore or emit error?
				}
				ls.consumeTemp(rhs)
				return
			}

			lhs := ls.lowerLValue(s.LHS[0])
			rhs := ls.lowerExpr(s.RHS[0])

			// Check if this is a global variable assignment
			isGlobal := false
			if lhsName, ok := s.LHS[0].(*ast.Ident); ok {
				if ls.globals != nil && ls.globals[lhsName.Name] {
					isGlobal = true
				}
			}

			// Check if this is a mutable variable assignment
			isMutable := false
			if lhsName, ok := s.LHS[0].(*ast.Ident); ok {
				if ls.isMutable(lhsName.Name) {
					isMutable = true
				}
			}

			if isGlobal {
				// Emit Store to global variable
				if lhsName, ok := s.LHS[0].(*ast.Ident); ok {
					ls.b.Emit(&hir.Store{
						Dst: hir.Var{Name: "@" + lhsName.Name},
						Val: rhs,
					})
					ls.consumeTemp(rhs)
				}
			} else if isMutable {
				// Emit Store for mutable variables
				ls.b.Emit(&hir.Store{
					Dst: hir.Var{Name: lhs},
					Val: rhs,
				})
				ls.consumeTemp(rhs) // Store consumes the value
			} else {
				// Emit Assign for SSA-style variables
				ls.b.Emit(&hir.Assign{LHS: lhs, RHS: rhs})
				ls.consumeTemp(rhs) // Assign consumes the value
			}
		}

	case *ast.AugAssignStmt:
		// Lower aug-assign: x += y becomes x = x + y
		// For mutable stack variables: load, binop, store
		if id, ok := s.Left.(*ast.Ident); ok {
			varName := id.Name

			// Load current value
			loadTemp := ls.b.FreshTemp("aug_load")
			if ls.isMutable(varName) {
				ls.b.Emit(&hir.Load{Type: "i32", Src: hir.Var{Name: varName}, Dst: loadTemp, DesiType: nil})
			} else {
				// For SSA-style variables, use the current value directly
				loadTemp = hir.Temp{Name: "%" + varName}
			}

			// Lower the RHS
			rhs := ls.lowerExpr(s.Right)

			// Extract operator from aug op (e.g., "+=" -> "+")
			op := strings.TrimSuffix(s.Op, "=")

			// Emit binary operation
			resultTemp := ls.b.FreshTemp("aug_result")
			ls.b.Emit(&hir.BinaryOp{Dst: resultTemp, Op: op, LHS: loadTemp, RHS: rhs, Type: "i32"})

			// Store result back
			if ls.isMutable(varName) {
				ls.b.Emit(&hir.Store{Dst: hir.Var{Name: varName}, Val: resultTemp})
			} else {
				ls.b.Emit(&hir.Assign{LHS: varName, RHS: resultTemp})
			}
			ls.consumeTemp(rhs)
		}

	case *ast.ExprStmt:
		_ = ls.lowerExpr(s.Expr) // materialize side effects if needed

	case *ast.MatchExpr:
		_ = ls.lowerMatchExpr(s)

	case *ast.SelectStmt:
		ls.lowerSelectStmt(s)

	case *ast.PassStmt:
		// pass is a no-op - nothing to emit

	case *ast.UnsafeBlock:
		// unsafe block: just lower the inner statements (unsafe is a type-checker concept)
		for _, stmt := range s.Body.Stmts {
			if ls.terminated {
				break
			}
			ls.lowerStmt(stmt)
		}

	case *ast.ReturnStmt:
		// IMPORTANT: Evaluate return expression FIRST, before any drops!
		// If return expression references a local that gets freed, we need
		// to capture its value before freeing.
		var v hir.Value
		if s.Value != nil {
			v = ls.lowerExpr(s.Value)
			ls.consumeTemp(v) // Return consumes the value

			// Type coercion: if return type is i64/u64 but expression type differs, cast
			funcRetType := ls.b.Func().RetType
			if funcRetType == "i64" {
				// Check if expression type needs widening to i64
				needsCast := false
				if ls.info != nil {
					exprType := ls.info.Types[s.Value]
					// Cast if type is int, i32, bool, or nil (unknown, likely i32)
					if exprType == nil || exprType == types.Int || exprType == types.I32 || exprType == types.Bool {
						needsCast = true
					}
				} else {
					// No type info - assume we need cast
					needsCast = true
				}
				if needsCast {
					casted := ls.b.FreshTemp("ret_i64")
					ls.b.Emit(&hir.Cast{Dst: casted, Src: v, Type: "i64"})
					v = casted
				}
			}

			// If returning a variable (not a field or expression), mark it moved
			if id, ok := s.Value.(*ast.Ident); ok {
				for i := len(ls.scopes) - 1; i >= 0; i-- {
					ls.scopes[i].moved[id.Name] = true
				}
			}
		}
		// THEN run defers and drop locals from all open scopes (inner→outer).
		// The return value is already captured, so we can safely free locals.
		ls.emitAllDefersAndDrops()
		ls.b.Emit(&hir.Ret{Val: v})
		ls.terminated = true

	case *ast.IfStmt:
		cond := ls.lowerExpr(s.Cond)

		// Check if condition needs __bool__ conversion
		if ls.info != nil {
			if cls, needsBool := ls.info.BoolConversions[s.Cond]; needsBool {
				// Emit call to ClassName___bool__(cond) -> i1
				boolResult := ls.b.FreshTemp("bool_result")
				mangledName := fmt.Sprintf("%s___bool__", cls.Name)
				ls.b.Emit(&hir.Call{
					Dst:  boolResult,
					Fn:   mangledName,
					Args: []hir.Value{cond},
					Type: "i1",
				})
				cond = boolResult
			}
		}

		thenBlk := ls.b.NewBlock("then")
		oldCur := ls.b.Block()

		// Save terminated state
		wasTerminated := ls.terminated
		ls.terminated = false // Start fresh for the block

		ls.push()
		ls.b.SetBlock(thenBlk)

		// Check if condition is IsExpr with bindings - extract payload in then block
		if isExpr, ok := s.Cond.(*ast.IsExpr); ok && ls.info != nil && !isExpr.Negated {
			if bindings := ls.info.IsBindings[isExpr]; len(bindings) > 0 {
				// Get the LHS value (previously lowered in lowerExpr)
				lhsVal := ls.lowerExpr(isExpr.X)

				// Load payload pointer from enum (offset 4, after the i32 tag)
				payloadPtrSlot := ls.b.FreshTemp("payload_ptr_slot")
				ls.b.Emit(&hir.GetElementPtr{
					Type:    "i8",
					Base:    lhsVal,
					Indices: []hir.Value{hir.ConstInt{Text: "4"}},
					Dst:     payloadPtrSlot,
				})

				payloadPtr := ls.b.FreshTemp("payload_ptr")
				ls.b.Emit(&hir.Load{
					Type: "ptr",
					Src:  payloadPtrSlot,
					Dst:  payloadPtr,
				})

				// For each binding, load the field value
				for _, binding := range bindings {
					val := ls.b.FreshTemp(binding.Name)
					ls.b.Emit(&hir.Load{
						Type: lowerType(binding.Type),
						Src:  payloadPtr,
						Dst:  val,
					})

					// Store in matchLocals for use in then body
					ls.matchLocals[binding.Name] = val
				}
			}
		}

		ls.lowerBlock(s.Then)
		scThen := ls.pop()
		if !ls.terminated {
			ls.emitScopeDrops(scThen)
		}
		thenTerminated := ls.terminated
		ls.b.SetBlock(oldCur)

		var elseBlk *hir.Block
		elseTerminated := false
		if s.Else != nil {
			elseBlk = ls.b.NewBlock("else")

			ls.terminated = false // Start fresh for the block

			ls.push()
			ls.b.SetBlock(elseBlk)
			ls.lowerBlock(s.Else)
			scElse := ls.pop()
			if !ls.terminated {
				ls.emitScopeDrops(scElse)
			}
			elseTerminated = ls.terminated
			ls.b.SetBlock(oldCur)

			// If both branches terminate, the if statement terminates
			ls.terminated = wasTerminated || (thenTerminated && elseTerminated)
		} else {
			// If no else, execution continues (unless already terminated before)
			ls.terminated = wasTerminated
		}

		ls.b.Emit(&hir.If{Cond: cond, Then: thenBlk, Else: elseBlk})

	case *ast.WhileStmt:
		// Create a condition block that will be re-evaluated each iteration
		condBlk := ls.b.NewBlock("while_cond")
		oldCur := ls.b.Block()

		// Lower condition in the condition block
		ls.b.SetBlock(condBlk)
		cond := ls.lowerExpr(s.Cond)
		ls.b.SetBlock(oldCur)

		// Create body block
		bodyBlk := ls.b.NewBlock("while_body")
		ls.push()
		ls.b.SetBlock(bodyBlk)
		ls.lowerBlock(s.Body)
		scWhile := ls.pop()
		if !ls.terminated {
			ls.emitScopeDrops(scWhile)
		}
		ls.b.SetBlock(oldCur)

		// Emit While with both condition block and body block
		ls.b.Emit(&hir.While{Cond: cond, CondBlock: condBlk, Body: bodyBlk})

	case *ast.ForStmt:
		// Desugar for-loop to while loop with index
		// Supports:
		// - List iteration: for x: int in items:
		// - Dict iteration: for key: str, value: int in my_dict.items():
		// - Enumerate: for i: int, x: str in enumerate(items):
		// - Reversed: for x: int in reversed(items):
		// - Tuple iteration (homogeneous): for x in (1, 2, 3): (compile-time unrolled)

		// Check for tuple iteration FIRST (before pushing outer scope)
		// This is handled specially because we unroll at compile time
		// EXCEPTION: Skip if this is a zip() call - zip returns tuple type for type-checking
		// but should be handled by the runtime zip iteration path below
		isZipCall := false
		if call, ok := s.Iter.(*ast.CallExpr); ok {
			if id, ok := call.Callee.(*ast.Ident); ok && id.Name == "zip" {
				isZipCall = true
			}
		}
		if ls.info != nil && !isZipCall {
			if tupType, ok := ls.info.Types[s.Iter].(*types.Tuple); ok && len(s.Targets) >= 1 && len(tupType.Elems) > 0 {
				// Tuple iteration: compile-time loop unrolling
				tupleVal := ls.lowerExpr(s.Iter)

				// Build struct type string for the tuple
				elemTypes := make([]string, len(tupType.Elems))
				for i := range tupType.Elems {
					elemTypes[i] = "ptr"
				}
				structType := "{" + strings.Join(elemTypes, ", ") + "}"

				// Unroll: emit body once for each element
				for idx := range tupType.Elems {
					ls.push()

					elemType := tupType.Elems[idx]
					llvmElemType := lowerType(elemType)

					elemPtr := ls.b.FreshTemp(fmt.Sprintf("tup_iter_%d_ptr", idx))
					ls.b.Emit(&hir.GetElementPtr{
						Type:    structType,
						Base:    tupleVal,
						Indices: []hir.Value{hir.ConstInt{Text: "0"}, hir.ConstInt{Text: fmt.Sprintf("%d", idx)}},
						Dst:     elemPtr,
					})

					boxed := ls.b.FreshTemp(fmt.Sprintf("tup_iter_%d_boxed", idx))
					ls.b.Emit(&hir.Load{Type: "ptr", Src: elemPtr, Dst: boxed})

					elemVal := ls.b.FreshTemp(fmt.Sprintf("tup_iter_%d_val", idx))
					ls.b.Emit(&hir.Load{Type: llvmElemType, Src: boxed, Dst: elemVal})

					if s.Targets[0].Name != nil {
						varName := s.Targets[0].Name.Name
						ls.cur().locals = append(ls.cur().locals, varName)
						ls.b.Emit(&hir.Let{Name: varName, Init: elemVal, Type: elemType})
						ls.cur().types[varName] = elemType
					}

					if s.Body != nil {
						ls.lowerBlock(s.Body)
					}

					scIter := ls.pop()
					if !ls.terminated {
						ls.emitScopeDrops(scIter)
					}
				}
				return // Return after tuple iteration - no outer scope was pushed
			}
		}

		ls.push()

		// Check if this is an enumerate() call
		isEnumerate := false
		var enumerateIterVal hir.Value
		if fe, ok := s.Iter.(*ast.CallExpr); ok {
			if callee, ok := fe.Callee.(*ast.Ident); ok && callee.Name == "enumerate" {
				if len(fe.Args) == 1 {
					isEnumerate = true
					enumerateIterVal = ls.lowerExpr(fe.Args[0])
				}
			}
		}

		// Check if this is a reversed() call
		isReversed := false
		var reversedIterVal hir.Value
		if fe, ok := s.Iter.(*ast.CallExpr); ok {
			if callee, ok := fe.Callee.(*ast.Ident); ok && callee.Name == "reversed" {
				if len(fe.Args) == 1 {
					isReversed = true
					reversedIterVal = ls.lowerExpr(fe.Args[0])
				}
			}
		}

		// Check if this is a zip(a, b) call
		isZip := false
		var zipIterVal1, zipIterVal2 hir.Value
		var zipElemType1, zipElemType2 types.T // element types for the two lists
		if fe, ok := s.Iter.(*ast.CallExpr); ok {
			if callee, ok := fe.Callee.(*ast.Ident); ok && callee.Name == "zip" {
				if len(fe.Args) == 2 {
					isZip = true
					zipIterVal1 = ls.lowerExpr(fe.Args[0])
					zipIterVal2 = ls.lowerExpr(fe.Args[1])
					// Get element types from list types
					if ls.info != nil {
						if list1, ok := ls.info.Types[fe.Args[0]].(*types.List); ok {
							zipElemType1 = list1.Elem
						}
						if list2, ok := ls.info.Types[fe.Args[1]].(*types.List); ok {
							zipElemType2 = list2.Elem
						}
					}
				}
			}
		}

		// Handle enumerate() with two targets (index, element)
		if isEnumerate && len(s.Targets) == 2 {
			// Get list length
			lenTemp := ls.b.FreshTemp("for_len")
			ls.b.Emit(&hir.Call{Dst: lenTemp, Fn: "list_len", Args: []hir.Value{enumerateIterVal}, Type: "i64"})

			// Allocate index variable
			idxPtr := ls.b.FreshTemp("for_idx_ptr")
			ls.b.Emit(&hir.Alloca{Dst: idxPtr, Type: "i64", Count: 1})
			ls.b.Emit(&hir.Store{Dst: idxPtr, Val: hir.ConstInt{Text: "0", Type: "i64"}})

			// Condition block
			condBlk := ls.b.NewBlock("for_cond")
			oldCur := ls.b.Block()

			ls.b.SetBlock(condBlk)
			idxVal := ls.b.FreshTemp("for_idx")
			ls.b.Emit(&hir.Load{Type: "i64", Src: idxPtr, Dst: idxVal})
			condTemp := ls.b.FreshTemp("for_cond")
			ls.b.Emit(&hir.BinaryOp{Dst: condTemp, Op: "<", LHS: idxVal, RHS: lenTemp, Type: "i1"})
			ls.b.SetBlock(oldCur)

			// Body block
			bodyBlk := ls.b.NewBlock("for_body")
			ls.b.SetBlock(bodyBlk)

			// Load current index for use in loop
			idxBody := ls.b.FreshTemp("for_idx_body")
			ls.b.Emit(&hir.Load{Type: "i64", Src: idxPtr, Dst: idxBody})

			// Cast i64 to i32 for list_get
			idxI32 := ls.b.FreshTemp("for_idx_i32")
			ls.b.Emit(&hir.Cast{Dst: idxI32, Src: idxBody, Type: "i32"})

			// Bind first target (index) - cast to i32 for Desi int
			if s.Targets[0].Name != nil {
				idxNameVal := ls.b.FreshTemp(s.Targets[0].Name.Name + "_val")
				ls.b.Emit(&hir.Cast{Dst: idxNameVal, Src: idxBody, Type: "i32"})
				ls.b.Emit(&hir.Let{Name: s.Targets[0].Name.Name, Init: idxNameVal, Type: types.Int})
			}

			// Get element: list_get(list, idx)
			elemPtr := ls.b.FreshTemp("elem_ptr")
			ls.b.Emit(&hir.Call{Dst: elemPtr, Fn: "list_get", Args: []hir.Value{enumerateIterVal, idxI32}, Type: "ptr"})

			// Bind second target (element)
			if s.Targets[1].Name != nil {
				ls.b.Emit(&hir.Let{Name: s.Targets[1].Name.Name, Init: elemPtr})
			}

			// Lower body
			if s.Body != nil {
				ls.lowerBlock(s.Body)
			}

			// Scope cleanup
			scFor := ls.pop()
			if !ls.terminated {
				ls.emitScopeDrops(scFor)
			}

			// Increment index
			incTemp := ls.b.FreshTemp("for_inc")
			ls.b.Emit(&hir.BinaryOp{Dst: incTemp, Op: "+", LHS: idxBody, RHS: hir.ConstInt{Text: "1", Type: "i64"}, Type: "i64"})
			ls.b.Emit(&hir.Store{Dst: idxPtr, Val: incTemp})

			ls.b.SetBlock(oldCur)
			ls.b.Emit(&hir.While{Cond: condTemp, CondBlock: condBlk, Body: bodyBlk})
			return
		}

		// Handle reversed() iteration - iterate from len-1 down to 0
		if isReversed && len(s.Targets) >= 1 {
			// Get list length
			lenTemp := ls.b.FreshTemp("for_len")
			ls.b.Emit(&hir.Call{Dst: lenTemp, Fn: "list_len", Args: []hir.Value{reversedIterVal}, Type: "i64"})

			// Allocate index variable, start at len-1
			idxPtr := ls.b.FreshTemp("for_idx_ptr")
			ls.b.Emit(&hir.Alloca{Dst: idxPtr, Type: "i64", Count: 1})
			// idx = len - 1
			startIdx := ls.b.FreshTemp("start_idx")
			ls.b.Emit(&hir.BinaryOp{Dst: startIdx, Op: "-", LHS: lenTemp, RHS: hir.ConstInt{Text: "1", Type: "i64"}, Type: "i64"})
			ls.b.Emit(&hir.Store{Dst: idxPtr, Val: startIdx})

			// Condition block: idx >= 0
			condBlk := ls.b.NewBlock("for_cond")
			oldCur := ls.b.Block()

			ls.b.SetBlock(condBlk)
			idxVal := ls.b.FreshTemp("for_idx")
			ls.b.Emit(&hir.Load{Type: "i64", Src: idxPtr, Dst: idxVal})
			condTemp := ls.b.FreshTemp("for_cond")
			// Compare: idx >= 0
			ls.b.Emit(&hir.BinaryOp{Dst: condTemp, Op: ">=", LHS: idxVal, RHS: hir.ConstInt{Text: "0", Type: "i64"}, Type: "i1"})
			ls.b.SetBlock(oldCur)

			// Body block
			bodyBlk := ls.b.NewBlock("for_body")
			ls.b.SetBlock(bodyBlk)

			// Load current index
			idxBody := ls.b.FreshTemp("for_idx_body")
			ls.b.Emit(&hir.Load{Type: "i64", Src: idxPtr, Dst: idxBody})

			// Cast to i32 for list_get
			idxI32 := ls.b.FreshTemp("for_idx_i32")
			ls.b.Emit(&hir.Cast{Dst: idxI32, Src: idxBody, Type: "i32"})

			// Get element
			elemPtr := ls.b.FreshTemp("elem_ptr")
			ls.b.Emit(&hir.Call{Dst: elemPtr, Fn: "list_get", Args: []hir.Value{reversedIterVal, idxI32}, Type: "ptr"})

			// Cast to element type (i32 for int lists)
			elemTemp := ls.b.FreshTemp("for_elem")
			ls.b.Emit(&hir.Cast{Dst: elemTemp, Src: elemPtr, Type: "i32"})

			// Bind loop variable
			if s.Targets[0].Name != nil {
				ls.b.Emit(&hir.Let{Name: s.Targets[0].Name.Name, Init: elemTemp})
			}

			// Lower body
			if s.Body != nil {
				ls.lowerBlock(s.Body)
			}

			// Scope cleanup
			scFor := ls.pop()
			if !ls.terminated {
				ls.emitScopeDrops(scFor)
			}

			// Decrement index: idx = idx - 1
			decTemp := ls.b.FreshTemp("for_dec")
			ls.b.Emit(&hir.BinaryOp{Dst: decTemp, Op: "-", LHS: idxBody, RHS: hir.ConstInt{Text: "1", Type: "i64"}, Type: "i64"})
			ls.b.Emit(&hir.Store{Dst: idxPtr, Val: decTemp})

			ls.b.SetBlock(oldCur)
			ls.b.Emit(&hir.While{Cond: condTemp, CondBlock: condBlk, Body: bodyBlk})
			return
		}

		// Handle zip(a, b) with two targets - parallel iteration
		if isZip && len(s.Targets) == 2 {
			// Get length of first list (iteration stops at min length - checking bounds in list_get)
			lenTemp := ls.b.FreshTemp("for_len")
			ls.b.Emit(&hir.Call{Dst: lenTemp, Fn: "list_len", Args: []hir.Value{zipIterVal1}, Type: "i64"})

			// Allocate index variable
			idxPtr := ls.b.FreshTemp("for_idx_ptr")
			ls.b.Emit(&hir.Alloca{Dst: idxPtr, Type: "i64", Count: 1})
			ls.b.Emit(&hir.Store{Dst: idxPtr, Val: hir.ConstInt{Text: "0", Type: "i64"}})

			// Condition block
			condBlk := ls.b.NewBlock("for_cond")
			oldCur := ls.b.Block()

			ls.b.SetBlock(condBlk)
			idxVal := ls.b.FreshTemp("for_idx")
			ls.b.Emit(&hir.Load{Type: "i64", Src: idxPtr, Dst: idxVal})
			condTemp := ls.b.FreshTemp("for_cond")
			ls.b.Emit(&hir.BinaryOp{Dst: condTemp, Op: "<", LHS: idxVal, RHS: lenTemp, Type: "i1"})
			ls.b.SetBlock(oldCur)

			// Body block
			bodyBlk := ls.b.NewBlock("for_body")
			ls.b.SetBlock(bodyBlk)

			// Load current index
			idxBody := ls.b.FreshTemp("for_idx_body")
			ls.b.Emit(&hir.Load{Type: "i64", Src: idxPtr, Dst: idxBody})

			// Cast to i32 for list_get
			idxI32 := ls.b.FreshTemp("for_idx_i32")
			ls.b.Emit(&hir.Cast{Dst: idxI32, Src: idxBody, Type: "i32"})

			// Get element from first list
			elem1Ptr := ls.b.FreshTemp("elem1_ptr")
			ls.b.Emit(&hir.Call{Dst: elem1Ptr, Fn: "list_get", Args: []hir.Value{zipIterVal1, idxI32}, Type: "ptr"})
			elem1 := ls.b.FreshTemp("elem1")
			elem1Type := "i32" // default
			if zipElemType1 != nil {
				elem1Type = lowerType(zipElemType1)
			}
			ls.b.Emit(&hir.Cast{Dst: elem1, Src: elem1Ptr, Type: elem1Type})

			// Get element from second list
			elem2Ptr := ls.b.FreshTemp("elem2_ptr")
			ls.b.Emit(&hir.Call{Dst: elem2Ptr, Fn: "list_get", Args: []hir.Value{zipIterVal2, idxI32}, Type: "ptr"})
			elem2 := ls.b.FreshTemp("elem2")
			elem2Type := "i32" // default
			if zipElemType2 != nil {
				elem2Type = lowerType(zipElemType2)
			}
			ls.b.Emit(&hir.Cast{Dst: elem2, Src: elem2Ptr, Type: elem2Type})

			// Bind first loop variable
			if s.Targets[0].Name != nil {
				ls.b.Emit(&hir.Let{Name: s.Targets[0].Name.Name, Init: elem1, Type: zipElemType1})
			}

			// Bind second loop variable
			if s.Targets[1].Name != nil {
				ls.b.Emit(&hir.Let{Name: s.Targets[1].Name.Name, Init: elem2, Type: zipElemType2})
			}

			// Lower body
			if s.Body != nil {
				ls.lowerBlock(s.Body)
			}

			// Scope cleanup
			scFor := ls.pop()
			if !ls.terminated {
				ls.emitScopeDrops(scFor)
			}

			// Increment index
			incTemp := ls.b.FreshTemp("for_inc")
			ls.b.Emit(&hir.BinaryOp{Dst: incTemp, Op: "+", LHS: idxBody, RHS: hir.ConstInt{Text: "1", Type: "i64"}, Type: "i64"})
			ls.b.Emit(&hir.Store{Dst: idxPtr, Val: incTemp})

			ls.b.SetBlock(oldCur)
			ls.b.Emit(&hir.While{Cond: condTemp, CondBlock: condBlk, Body: bodyBlk})
			return
		}

		// Check if this is a dict.items() call
		isDictItems := false
		var dictVal hir.Value
		if fe, ok := s.Iter.(*ast.CallExpr); ok {
			if field, ok := fe.Callee.(*ast.FieldExpr); ok {
				if field.Name.Name == "items" && ls.info != nil {
					if dictType := ls.info.Types[field.X]; dictType != nil {
						if _, isDict := dictType.(*types.Dict); isDict {
							isDictItems = true
							dictVal = ls.lowerExpr(field.X)
						}
					}
				}
			}
		}

		if isDictItems && len(s.Targets) == 2 {
			// Dict iteration: for key, value in dict.items():
			// Uses dict_keys() to get keys array, then iterates

			// Get keys array and count
			keyCountPtr := ls.b.FreshTemp("key_count_ptr")
			ls.b.Emit(&hir.Alloca{Dst: keyCountPtr, Type: "i64", Count: 1})
			keysPtr := ls.b.FreshTemp("keys_ptr")
			ls.b.Emit(&hir.Call{Dst: keysPtr, Fn: "dict_keys", Args: []hir.Value{dictVal, keyCountPtr}, Type: "ptr"})

			// Load key count
			lenTemp := ls.b.FreshTemp("dict_len")
			ls.b.Emit(&hir.Load{Type: "i64", Src: keyCountPtr, Dst: lenTemp})

			// Allocate index variable
			idxPtr := ls.b.FreshTemp("for_idx_ptr")
			ls.b.Emit(&hir.Alloca{Dst: idxPtr, Type: "i64", Count: 1})
			ls.b.Emit(&hir.Store{Dst: idxPtr, Val: hir.ConstInt{Text: "0", Type: "i64"}})

			// Condition block
			condBlk := ls.b.NewBlock("for_cond")
			oldCur := ls.b.Block()

			ls.b.SetBlock(condBlk)
			idxVal := ls.b.FreshTemp("for_idx")
			ls.b.Emit(&hir.Load{Type: "i64", Src: idxPtr, Dst: idxVal})
			condTemp := ls.b.FreshTemp("for_cond")
			ls.b.Emit(&hir.BinaryOp{Dst: condTemp, Op: "<", LHS: idxVal, RHS: lenTemp, Type: "i1"})
			ls.b.SetBlock(oldCur)

			// Body block
			bodyBlk := ls.b.NewBlock("for_body")
			ls.b.SetBlock(bodyBlk)

			// Load current index
			idxBody := ls.b.FreshTemp("for_idx_body")
			ls.b.Emit(&hir.Load{Type: "i64", Src: idxPtr, Dst: idxBody})

			// Cast to i32 for GEP
			idxBodyI32 := ls.b.FreshTemp("for_idx_i32")
			ls.b.Emit(&hir.Cast{Dst: idxBodyI32, Src: idxBody, Type: "i32"})

			// Get key: keys_ptr[idx]
			keyPtr := ls.b.FreshTemp("key_ptr")
			ls.b.Emit(&hir.GetElementPtr{Type: "ptr", Base: keysPtr, Indices: []hir.Value{idxBodyI32}, Dst: keyPtr})
			keyVal := ls.b.FreshTemp("key_val")
			ls.b.Emit(&hir.Load{Type: "ptr", Src: keyPtr, Dst: keyVal})

			// Get value: dict_get(dict, key_int, key_str, key_float, key_ptr, default)
			// For dict.items() with string keys, keyVal is the string key
			// Note: dict_keys() returns string representations for all key types
			// So we use keyVal as key_str and pass 0 for key_int/key_float
			nullPtr := ls.b.FreshTemp("null_ptr")
			ls.b.Emit(&hir.Alloca{Dst: nullPtr, Type: "i64", Count: 1})
			ls.b.Emit(&hir.Store{Dst: nullPtr, Val: hir.ConstInt{Text: "0", Type: "i64"}})

			// For string-keyed dicts (currently only working case for dict.items())
			keyInt := hir.ConstInt{Text: "0", Type: "i64"}
			keyFloat := hir.ConstFloat{Text: "0.0"}
			var customKeyPtr hir.Value = hir.ConstNull{}
			valuePtr := ls.b.FreshTemp("value_ptr")
			ls.b.Emit(&hir.Call{Dst: valuePtr, Fn: "dict_get", Args: []hir.Value{dictVal, keyInt, keyVal, keyFloat, customKeyPtr, nullPtr}, Type: "ptr"})

			// Load value (as i64, then cast to i32 for int)
			valueI64 := ls.b.FreshTemp("value_i64")
			ls.b.Emit(&hir.Load{Type: "i64", Src: valuePtr, Dst: valueI64})
			valueVal := ls.b.FreshTemp("value_val")
			ls.b.Emit(&hir.Cast{Dst: valueVal, Src: valueI64, Type: "i32"})

			// Bind key variable (first target)
			if s.Targets[0].Name != nil {
				ls.b.Emit(&hir.Let{Name: s.Targets[0].Name.Name, Init: keyVal})
			}
			// Bind value variable (second target)
			if s.Targets[1].Name != nil {
				ls.b.Emit(&hir.Let{Name: s.Targets[1].Name.Name, Init: valueVal})
			}

			// Lower body
			if s.Body != nil {
				ls.lowerBlock(s.Body)
			}

			// Scope cleanup
			scFor := ls.pop()
			if !ls.terminated {
				ls.emitScopeDrops(scFor)
			}

			// Increment index
			incTemp := ls.b.FreshTemp("for_inc")
			ls.b.Emit(&hir.BinaryOp{Dst: incTemp, Op: "+", LHS: idxBody, RHS: hir.ConstInt{Text: "1", Type: "i64"}, Type: "i64"})
			ls.b.Emit(&hir.Store{Dst: idxPtr, Val: incTemp})

			ls.b.SetBlock(oldCur)
			ls.b.Emit(&hir.While{Cond: condTemp, CondBlock: condBlk, Body: bodyBlk})

			// Free keys array after loop (keys_ptr was malloc'd by dict_keys)
			ls.b.Emit(&hir.Call{Fn: "free", Args: []hir.Value{keysPtr}})

		} else {

			// Check if this is set iteration
			isSet := false
			if ls.info != nil {
				iterType := ls.info.Types[s.Iter]
				if _, ok := iterType.(*types.Set); ok {
					isSet = true
				}
			}

			if isSet && len(s.Targets) >= 1 {
				// Set iteration: convert set to array and iterate
				setVal := ls.lowerExpr(s.Iter)

				// Get array and length from set: set_to_array(set, &len) -> ptr
				lenPtr := ls.b.FreshTemp("set_len_ptr")
				ls.b.Emit(&hir.Alloca{Dst: lenPtr, Type: "i64", Count: 1})
				arrPtr := ls.b.FreshTemp("set_arr")
				ls.b.Emit(&hir.Call{Dst: arrPtr, Fn: "set_to_array", Args: []hir.Value{setVal, lenPtr}, Type: "ptr"})

				// Load length
				lenTemp := ls.b.FreshTemp("set_len")
				ls.b.Emit(&hir.Load{Type: "i64", Src: lenPtr, Dst: lenTemp})

				// Allocate index variable
				idxPtr := ls.b.FreshTemp("for_idx_ptr")
				ls.b.Emit(&hir.Alloca{Dst: idxPtr, Type: "i64", Count: 1})
				ls.b.Emit(&hir.Store{Dst: idxPtr, Val: hir.ConstInt{Text: "0", Type: "i64"}})

				// Condition block
				condBlk := ls.b.NewBlock("for_cond")
				oldCur := ls.b.Block()

				ls.b.SetBlock(condBlk)
				idxVal := ls.b.FreshTemp("for_idx")
				ls.b.Emit(&hir.Load{Type: "i64", Src: idxPtr, Dst: idxVal})
				condTemp := ls.b.FreshTemp("for_cond")
				ls.b.Emit(&hir.BinaryOp{Dst: condTemp, Op: "<", LHS: idxVal, RHS: lenTemp, Type: "i1"})
				ls.b.SetBlock(oldCur)

				// Body block
				bodyBlk := ls.b.NewBlock("for_body")
				ls.b.SetBlock(bodyBlk)

				// Load current index
				idxBody := ls.b.FreshTemp("for_idx_body")
				ls.b.Emit(&hir.Load{Type: "i64", Src: idxPtr, Dst: idxBody})

				// Cast to i32 for GEP
				idxBodyI32 := ls.b.FreshTemp("for_idx_i32")
				ls.b.Emit(&hir.Cast{Dst: idxBodyI32, Src: idxBody, Type: "i32"})

				// Get element from array: arr[idx]
				// arrPtr is int64_t*, so GEP with index
				elemPtr := ls.b.FreshTemp("elem_ptr")
				ls.b.Emit(&hir.GetElementPtr{Type: "i64", Base: arrPtr, Indices: []hir.Value{idxBodyI32}, Dst: elemPtr})
				elemVal := ls.b.FreshTemp("elem_val")
				ls.b.Emit(&hir.Load{Type: "i64", Src: elemPtr, Dst: elemVal})

				// Cast to i32 for Desi int
				elemI32 := ls.b.FreshTemp("for_elem")
				ls.b.Emit(&hir.Cast{Dst: elemI32, Src: elemVal, Type: "i32"})

				// Bind loop variable
				if s.Targets[0].Name != nil {
					ls.b.Emit(&hir.Let{Name: s.Targets[0].Name.Name, Init: elemI32})
				}

				// Lower body
				if s.Body != nil {
					ls.lowerBlock(s.Body)
				}

				// Scope cleanup
				scFor := ls.pop()
				if !ls.terminated {
					ls.emitScopeDrops(scFor)
				}

				// Increment index
				incTemp := ls.b.FreshTemp("for_inc")
				ls.b.Emit(&hir.BinaryOp{Dst: incTemp, Op: "+", LHS: idxBody, RHS: hir.ConstInt{Text: "1", Type: "i64"}, Type: "i64"})
				ls.b.Emit(&hir.Store{Dst: idxPtr, Val: incTemp})

				ls.b.SetBlock(oldCur)
				ls.b.Emit(&hir.While{Cond: condTemp, CondBlock: condBlk, Body: bodyBlk})

				// Free array after loop (arrPtr was malloc'd by set_to_array)
				ls.b.Emit(&hir.Call{Fn: "free", Args: []hir.Value{arrPtr}})
				return

			}

			// Check for custom class iteration via __getitem__ + __len__
			if ls.info != nil {
				if cls, ok := ls.info.Types[s.Iter].(*types.Class); ok {
					_, hasGetItem := cls.Dunders["__getitem__"]
					_, hasLen := cls.Dunders["__len__"]
					if hasGetItem && hasLen {
						// Custom class iteration using __getitem__/__len__
						objVal := ls.lowerExpr(s.Iter)

						// Get length via __len__
						lenMangledName := fmt.Sprintf("%s___len__", cls.Name)
						lenTemp := ls.b.FreshTemp("for_len")
						ls.b.Emit(&hir.Call{Dst: lenTemp, Fn: lenMangledName, Args: []hir.Value{objVal}, Type: "i32"})

						// Cast to i64 for comparison
						lenI64 := ls.b.FreshTemp("for_len_i64")
						ls.b.Emit(&hir.Cast{Dst: lenI64, Src: lenTemp, Type: "i64"})

						// Allocate index variable
						idxPtr := ls.b.FreshTemp("for_idx_ptr")
						ls.b.Emit(&hir.Alloca{Dst: idxPtr, Type: "i64", Count: 1})
						ls.b.Emit(&hir.Store{Dst: idxPtr, Val: hir.ConstInt{Text: "0", Type: "i64"}})

						// Condition block
						condBlk := ls.b.NewBlock("for_cond")
						oldCur := ls.b.Block()

						ls.b.SetBlock(condBlk)
						idxVal := ls.b.FreshTemp("for_idx")
						ls.b.Emit(&hir.Load{Type: "i64", Src: idxPtr, Dst: idxVal})
						condTemp := ls.b.FreshTemp("for_cond")
						ls.b.Emit(&hir.BinaryOp{Dst: condTemp, Op: "<", LHS: idxVal, RHS: lenI64, Type: "i1"})
						ls.b.SetBlock(oldCur)

						// Body block
						bodyBlk := ls.b.NewBlock("for_body")
						ls.b.SetBlock(bodyBlk)

						idxBody := ls.b.FreshTemp("for_idx_body")
						ls.b.Emit(&hir.Load{Type: "i64", Src: idxPtr, Dst: idxBody})

						// Cast to i32 for __getitem__
						idxI32 := ls.b.FreshTemp("for_idx_i32")
						ls.b.Emit(&hir.Cast{Dst: idxI32, Src: idxBody, Type: "i32"})

						// Get element via __getitem__
						getItemMangledName := fmt.Sprintf("%s___getitem__", cls.Name)
						elemTemp := ls.b.FreshTemp("for_elem")

						// Get return type from __getitem__
						elemLLVMType := "i32" // default
						var elemDesiType types.T = types.Int
						if getItemFn, ok := cls.Dunders["__getitem__"]; ok {
							if getItemFn.Ret != nil {
								elemDesiType = getItemFn.Ret
								elemLLVMType = lowerType(getItemFn.Ret)
							}
						}

						ls.b.Emit(&hir.Call{Dst: elemTemp, Fn: getItemMangledName, Args: []hir.Value{objVal, idxI32}, Type: elemLLVMType})

						// Bind loop variable
						if len(s.Targets) > 0 && s.Targets[0].Name != nil {
							ls.b.Emit(&hir.Let{Name: s.Targets[0].Name.Name, Init: elemTemp, Type: elemDesiType})
						}

						// Lower body
						if s.Body != nil {
							ls.lowerBlock(s.Body)
						}

						scFor := ls.pop()
						if !ls.terminated {
							ls.emitScopeDrops(scFor)
						}

						// Increment index
						incTemp := ls.b.FreshTemp("for_inc")
						ls.b.Emit(&hir.BinaryOp{Dst: incTemp, Op: "+", LHS: idxBody, RHS: hir.ConstInt{Text: "1", Type: "i64"}, Type: "i64"})
						ls.b.Emit(&hir.Store{Dst: idxPtr, Val: incTemp})

						ls.b.SetBlock(oldCur)
						ls.b.Emit(&hir.While{Cond: condTemp, CondBlock: condBlk, Body: bodyBlk})
						return
					}
				}
			}

			// Check for __iter__/__next__ iterator protocol
			if ls.info != nil {
				if cls, ok := ls.info.Types[s.Iter].(*types.Class); ok {
					_, hasIter := cls.Dunders["__iter__"]
					nextDunder, hasNext := cls.Dunders["__next__"]
					if hasIter && hasNext {
						// Custom iterator using __iter__/__next__ protocol
						// 1. Call __iter__ to get iterator
						// 2. Loop calling __next__ until it returns None/Nothing

						// Extract element type from __next__ return type (Option<T> -> T)
						var elemType types.T = types.Int // default fallback
						var elemLLVMType = "ptr"         // default to ptr for generics
						if nextDunder.Ret != nil {
							if innerType := types.OptionSomeType(nextDunder.Ret); innerType != nil {
								elemType = innerType
								elemLLVMType = lowerType(innerType)
							}
						}

						objVal := ls.lowerExpr(s.Iter)

						// Call __iter__ to get iterator
						iterMangledName := fmt.Sprintf("%s___iter__", cls.Name)
						iterTemp := ls.b.FreshTemp("iter_obj")
						ls.b.Emit(&hir.Call{Dst: iterTemp, Fn: iterMangledName, Args: []hir.Value{objVal}, Type: "ptr"})

						// Condition block - call __next__ and check if Some
						condBlk := ls.b.NewBlock("for_iter_cond")
						oldCur := ls.b.Block()

						ls.b.SetBlock(condBlk)
						// Call __next__
						nextMangledName := fmt.Sprintf("%s___next__", cls.Name)
						optionTemp := ls.b.FreshTemp("iter_option")
						ls.b.Emit(&hir.Call{Dst: optionTemp, Fn: nextMangledName, Args: []hir.Value{iterTemp}, Type: "ptr"})

						// Check is_some (Option variant tag == 0 means Some)
						// Option layout: tag (i32 at offset 0), payload ptr (ptr at offset 4)
						tagPtr := ls.b.FreshTemp("option_tag_ptr")
						ls.b.Emit(&hir.GetElementPtr{Type: "i8", Base: optionTemp, Indices: []hir.Value{hir.ConstInt{Text: "0"}}, Dst: tagPtr})
						tag := ls.b.FreshTemp("option_tag")
						ls.b.Emit(&hir.Load{Type: "i32", Src: tagPtr, Dst: tag})
						condTemp := ls.b.FreshTemp("has_value")
						ls.b.Emit(&hir.BinaryOp{Dst: condTemp, Op: "==", LHS: tag, RHS: hir.ConstInt{Text: "0", Type: "i32"}, Type: "i1"})
						ls.b.SetBlock(oldCur)

						// Body block
						bodyBlk := ls.b.NewBlock("for_iter_body")
						ls.b.SetBlock(bodyBlk)

						ls.push()

						// Extract value from Option (payload ptr is at offset 4)
						valPtrPtr := ls.b.FreshTemp("option_val_ptr_ptr")
						ls.b.Emit(&hir.GetElementPtr{Type: "i8", Base: optionTemp, Indices: []hir.Value{hir.ConstInt{Text: "4"}}, Dst: valPtrPtr})
						valPtr := ls.b.FreshTemp("option_val_ptr")
						ls.b.Emit(&hir.Load{Type: "ptr", Src: valPtrPtr, Dst: valPtr})
						elemVal := ls.b.FreshTemp("iter_elem")
						ls.b.Emit(&hir.Load{Type: elemLLVMType, Src: valPtr, Dst: elemVal})

						// Bind loop variable
						if len(s.Targets) >= 1 && s.Targets[0].Name != nil {
							ls.b.Emit(&hir.Let{Name: s.Targets[0].Name.Name, Init: elemVal, Type: elemType})
						}

						// Lower body
						if s.Body != nil {
							ls.lowerBlock(s.Body)
						}

						scFor := ls.pop()
						if !ls.terminated {
							ls.emitScopeDrops(scFor)
						}

						ls.b.SetBlock(oldCur)
						ls.b.Emit(&hir.While{Cond: condTemp, CondBlock: condBlk, Body: bodyBlk})
						return // Exit to prevent fall-through to list iteration path
					}
				}
			}

			// List iteration (original code)
			iterVal := ls.lowerExpr(s.Iter)

			// Get element type from list type info
			var elemLLVMType = "i32" // default
			var elemDesiType types.T = types.Int
			if ls.info != nil {
				if listT, ok := ls.info.Types[s.Iter].(*types.List); ok {
					elemDesiType = listT.Elem
					elemLLVMType = lowerType(listT.Elem)
				}
			}

			// Get length of collection
			lenTemp := ls.b.FreshTemp("for_len")
			ls.b.Emit(&hir.Call{Dst: lenTemp, Fn: "list_len", Args: []hir.Value{iterVal}, Type: "i64"})

			// Allocate stack space for index variable
			idxPtr := ls.b.FreshTemp("for_idx_ptr")
			ls.b.Emit(&hir.Alloca{Dst: idxPtr, Type: "i64", Count: 1})
			ls.b.Emit(&hir.Store{Dst: idxPtr, Val: hir.ConstInt{Text: "0", Type: "i64"}})

			// Create condition block
			condBlk := ls.b.NewBlock("for_cond")
			oldCur := ls.b.Block()

			ls.b.SetBlock(condBlk)
			idxVal := ls.b.FreshTemp("for_idx")
			ls.b.Emit(&hir.Load{Type: "i64", Src: idxPtr, Dst: idxVal, DesiType: nil})
			condTemp := ls.b.FreshTemp("for_cond")
			ls.b.Emit(&hir.BinaryOp{Dst: condTemp, Op: "<", LHS: idxVal, RHS: lenTemp, Type: "i1"})
			ls.b.SetBlock(oldCur)

			// Create body block
			bodyBlk := ls.b.NewBlock("for_body")
			ls.b.SetBlock(bodyBlk)

			idxBody := ls.b.FreshTemp("for_idx_body")
			ls.b.Emit(&hir.Load{Type: "i64", Src: idxPtr, Dst: idxBody, DesiType: nil})

			idxBodyI32 := ls.b.FreshTemp("for_idx_i32")
			ls.b.Emit(&hir.Cast{Dst: idxBodyI32, Src: idxBody, Type: "i32"})

			// Bind loop variable
			if len(s.Targets) > 0 {
				for _, tgt := range s.Targets {
					if tgt.Name != nil {
						elemPtrTemp := ls.b.FreshTemp("elem_ptr")
						ls.b.Emit(&hir.Call{Dst: elemPtrTemp, Fn: "list_get", Args: []hir.Value{iterVal, idxBodyI32}})

						elemTemp := ls.b.FreshTemp("for_elem")
						ls.b.Emit(&hir.Cast{Dst: elemTemp, Src: elemPtrTemp, Type: elemLLVMType})

						ls.b.Emit(&hir.Let{Name: tgt.Name.Name, Init: elemTemp, Type: elemDesiType})
					}
				}
			}

			// Lower body
			if s.Body != nil {
				ls.lowerBlock(s.Body)
			}

			scFor := ls.pop()
			if !ls.terminated {
				ls.emitScopeDrops(scFor)
			}

			// Increment index
			incTemp := ls.b.FreshTemp("for_inc")
			ls.b.Emit(&hir.BinaryOp{Dst: incTemp, Op: "+", LHS: idxBody, RHS: hir.ConstInt{Text: "1", Type: "i64"}, Type: "i64"})
			ls.b.Emit(&hir.Store{Dst: idxPtr, Val: incTemp})

			ls.b.SetBlock(oldCur)
			ls.b.Emit(&hir.While{Cond: condTemp, CondBlock: condBlk, Body: bodyBlk})
		}

	case *ast.UsingStmt:
		// using X [= init]: body  → bind handle + defer destroy_arena(X)
		ls.push()
		ident := ls.nameOf(s.Bind)
		if ident != "" {
			// Don't register as "local" (avoid ordinary drop); we destroy via defer.
			var initVal hir.Value
			if s.Init != nil {
				initVal = ls.lowerExpr(s.Init)
				ls.b.Emit(&hir.Let{Name: ident, Init: initVal})
			} else {
				// No explicit init - create a new arena with __arena_new
				arenaPtr := ls.b.FreshTemp("arena_new")
				ls.b.Emit(&hir.Call{
					Dst:  arenaPtr,
					Fn:   "__arena_new",
					Args: []hir.Value{hir.ConstInt{Text: "0", Type: "i64"}}, // 0 = default capacity (64KB)
					Type: "ptr",
				})
				initVal = arenaPtr
				ls.b.Emit(&hir.Let{Name: ident, Init: initVal, Type: types.ArenaOf()})
			}

			// Check if it's a class with __close__
			var closeMangledName string
			if s.Init != nil {
				if typ := ls.info.Types[s.Init]; typ != nil {
					if cls, ok := typ.(*types.Class); ok {
						// Look for __close__ in class or base classes
						curr := cls
						for curr != nil {
							if _, ok := curr.Dunders["__close__"]; ok {
								// Found __close__. Construct mangled name.
								// Name mangling: ClassName_methodName
								// Note: We use the class where the method is defined?
								// Or the class of the object?
								// Desi uses static dispatch. If we call obj.__close__(), it resolves to ClassName___close__.
								// If it's inherited, it resolves to BaseName___close__.
								// So we should use `curr.Name`.
								closeMangledName = fmt.Sprintf("%s___close__", curr.Name)
								break
							}
							curr = curr.Base
						}
					}
				}
			}

			if closeMangledName != "" {
				// RAII class: register for __close__ call
				ls.cur().closers[ident] = closeMangledName
			} else if s.Init != nil && isMutexGuardType(ls.info.Types[s.Init]) {
				// MutexGuard type: register for mutex_unlock at scope end
				ls.cur().mutexGuards[ident] = true
			} else if s.Init != nil && isReadGuardType(ls.info.Types[s.Init]) {
				// ReadGuard type: register for read_guard_unlock at scope end
				ls.cur().readGuards[ident] = true
			} else if s.Init != nil && isWriteGuardType(ls.info.Types[s.Init]) {
				// WriteGuard type: register for write_guard_unlock at scope end
				ls.cur().writeGuards[ident] = true
			} else if s.Init != nil && isFileType(ls.info.Types[s.Init]) {
				// File type: register for file_close at scope end
				ls.cur().files[ident] = true
			} else if s.Init != nil && isSenderType(ls.info.Types[s.Init]) {
				// Sender type: register for sender_drop at scope end
				ls.cur().senders[ident] = true
			} else if s.Init != nil && isReceiverType(ls.info.Types[s.Init]) {
				// Receiver type: register for receiver_drop at scope end
				ls.cur().receivers[ident] = true
			} else {
				// Fallback: Mark this name as an arena handle in the current scope.
				ls.cur().arenas[ident] = true
			}

			// One destroy at scope end (or return) via defer.
			ls.cur().defers = append(ls.cur().defers, hir.Var{Name: ident})
		}
		ls.lowerBlock(s.Body)
		sc := ls.pop()
		if !ls.terminated {
			ls.emitScopeDrops(sc)
		}

	case *ast.DeferStmt:
		// Keep the basic "__close(x)" → drop/decref path from M7A/B.
		if ce := s.Call; ce != nil {
			if id, ok := ce.Callee.(*ast.Ident); ok && id.Name == "__close" && len(ce.Args) == 1 {
				if v := ls.valueOf(ce.Args[0]); v != nil {
					ls.cur().defers = append(ls.cur().defers, v)
				}
			}
		}

	case *ast.SpawnStmt:
		// spawn: block → for now, inline the body as a placeholder
		// Full closure capture and scheduler integration will be added later
		// This allows the syntax to work while we build out the runtime

		// TODO: Phase 2 will add:
		// 1. Closure capture for free variables
		// 2. scheduler_spawn(fn_ptr, captured_ctx, name) call
		// 3. Proper task creation and scheduling

		// For now, just execute the body inline (sequential, not concurrent)
		if s.Body != nil {
			ls.lowerBlock(s.Body)
		}

	default:
		// other statements ignored for this phase
	}
}

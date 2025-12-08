package lower

import (
	"fmt"

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

			// Handle field assignment: obj.field = val
			if field, ok := s.LHS[0].(*ast.FieldExpr); ok {
				// Lower receiver
				recv := ls.lowerExpr(field.X)
				rhs := ls.lowerExpr(s.RHS[0])

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

			// Check if this is a mutable variable assignment
			isMutable := false
			if lhsName, ok := s.LHS[0].(*ast.Ident); ok {
				if ls.isMutable(lhsName.Name) {
					isMutable = true
				}
			}

			if isMutable {
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

	case *ast.ExprStmt:
		_ = ls.lowerExpr(s.Expr) // materialize side effects if needed

	case *ast.MatchExpr:
		_ = ls.lowerMatchExpr(s)

	case *ast.ReturnStmt:
		// Before returning, run defers and drop locals from all open scopes (inner→outer).
		ls.emitAllDefersAndDrops()
		var v hir.Value
		if s.Value != nil {
			v = ls.lowerExpr(s.Value)
			ls.consumeTemp(v) // Return consumes the value
		}
		ls.b.Emit(&hir.Ret{Val: v})
		ls.terminated = true

	case *ast.IfStmt:
		cond := ls.lowerExpr(s.Cond)

		thenBlk := ls.b.NewBlock("then")
		oldCur := ls.b.Block()

		// Save terminated state
		wasTerminated := ls.terminated
		ls.terminated = false // Start fresh for the block

		ls.push()
		ls.b.SetBlock(thenBlk)
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
				ls.b.Emit(&hir.Let{Name: ident, Init: initVal})
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
	default:
		// other statements ignored for this phase
	}
}

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
				ls.b.Emit(&hir.Let{Name: ident})
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

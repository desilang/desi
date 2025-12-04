package llvm

import (
	"fmt"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/backend/llvm/intrin"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/types"
)

func (m *Module) EmitFunc(fn *hir.Func) {
	// Mark this function as defined to avoid emitting a declare for it
	m.definedFunctions[fn.Name] = true

	// Reset per-function state.
	m.ssa = make(map[string]hir.Value)
	m.tempTypes = make(map[string]string)
	m.varTypes = make(map[string]types.T)
	m.curRetIsPtr = m.asyncWrappers[fn.Name]
	m.currentMoves = nil

	// Look up moves and param types if we have origin info
	if m.info != nil && fn.Origin != nil {
		if fd, ok := fn.Origin.(*ast.FuncDecl); ok {
			if moves, ok := m.info.FuncMoves[fd]; ok {
				m.currentMoves = moves
			}
			// Populate varTypes for parameters
			for i := range fd.Params {
				// Param.Name is a struct, so we take its address
				// The checker uses the address of the Ident in the AST
				if sym := m.info.Idents[&fd.Params[i].Name]; sym != nil {
					if sym.Type != nil {
						m.varTypes[fd.Params[i].Name.Name] = sym.Type
					}
				}
			}
		}
	}

	// --- header types (now typed-aware) ---
	// Default ret: i32 (Tier-0), but async wrappers return ptr (future handle).
	retTy := "i32"
	if m.curRetIsPtr {
		retTy = "ptr"
	}
	// Override from registered signature, unless this define is an async wrapper.
	if sig, ok := getFuncSig(fn.Name); ok && !m.curRetIsPtr && sig.ret != "" {
		retTy = sig.ret
	}
	// Explicit HIR override (M14)
	if fn.RetType != "" {
		retTy = fn.RetType
	}

	m.curFuncRetTy = retTy

	// Emit function header with per-param overrides (default ptr).
	wprintf(&m.funcs, "define %s @%s(", retTy, fn.Name)
	for i, p := range fn.Params {
		if i > 0 {
			wprintf(&m.funcs, ", ")
		}
		pty := p.Type // Use type from HIR if available
		if pty == "" {
			// Fallback to signature lookup
			pty = "ptr"
			if sig, ok := getFuncSig(fn.Name); ok && i < len(sig.params) && sig.params[i] != "" {
				pty = sig.params[i]
			}
		}
		wprintf(&m.funcs, "%s %%%s", pty, p.Name)
		// Track parameter type for operand() lookups
		m.tempTypes[p.Name] = pty
	}
	wprintf(&m.funcs, ") {\n")

	type localInfo struct {
		name string
		size int
	}

	// Pre-calculate unique labels for all blocks
	blockLabels := make(map[*hir.Block]string)
	labelCounts := make(map[string]int)
	for i, b := range fn.Blocks {
		name := b.Name
		if name == "" {
			name = "block"
		}
		if i == 0 {
			name = "entry"
		}

		// Uniquify
		count := labelCounts[name]
		uniqueName := name
		if count > 0 {
			uniqueName = fmt.Sprintf("%s%d", name, count)
		}
		labelCounts[name]++
		blockLabels[b] = uniqueName
	}

	for _, b := range fn.Blocks {
		label := blockLabels[b]
		wprintf(&m.funcs, "%s:\n", label)

		// Pre-seed simple SSA aliases for Let;Assign pairs (let x; x = ...).
		pre := make(map[string]hir.Value)
		for i := 0; i+1 < len(b.Stmts); i++ {
			lt, ok := b.Stmts[i].(*hir.Let)
			if !ok || lt.Init != nil {
				continue
			}
			if as, ok2 := b.Stmts[i+1].(*hir.Assign); ok2 && as.LHS == lt.Name {
				pre[lt.Name] = as.RHS
			}
		}
		for k, v := range pre {
			m.ssa[k] = v
		}

		var locals []localInfo
		lifetimesClosed := false // once we end-lifetime (e.g., before ret), don't do it again at block end

		for _, st := range b.Stmts {
			switch x := st.(type) {

			// ------- core statements -------
			case *hir.Let:
				// Track Desi type if available
				if x.Type != nil {
					if t, ok := x.Type.(types.T); ok {
						m.varTypes[x.Name] = t
					}
				}

				// If type is a reference type (set, dict, list, str) with an initializer,
				// use SSA directly without alloca (immutable variables)
				if isReferenceType(x.Type) && x.Init != nil {
					m.ssa[x.Name] = x.Init
					// Skip alloca for immutable reference types - they're already pointers
					continue
				}

				// For mutable reference types (Init == nil) or value types,
				// emit alloca for proper stack storage
				llvmTy, size := "ptr", 8 // Default to ptr for reference types
				if x.Init != nil {
					switch x.Init.(type) {
					case hir.ConstBool:
						llvmTy, size = "i1", 1
					case hir.ConstStr:
						llvmTy, size = "ptr", 8
					case hir.ConstInt:
						llvmTy, size = "i32", 4
					case hir.Temp:
						// For temps, default to i32 unless it's a  reference type
						llvmTy, size = "i32", 4
					}
					// Even with initializer, record SSA alias for convenience.
					m.ssa[x.Name] = x.Init
				} else if !isReferenceType(x.Type) {
					// Value types without initializer
					llvmTy, size = "i32", 4
				}
				wprintf(&m.funcs, "  %%%s = alloca %s\n", x.Name, llvmTy)
				wprintf(&m.funcs, "%s", intrin.LifetimeStart(size, x.Name))
				locals = append(locals, localInfo{name: x.Name, size: size})

			case *hir.Assign:
				m.ssa[x.LHS] = x.RHS

			case *hir.Call:
				m.emitCall(x)

			case *hir.BinaryOp:
				// Emit LLVM IR for binary operations
				lty, lval := m.operand(x.LHS)
				rty, rval := m.operand(x.RHS)

				// Special case: String concatenation with type conversions
				if x.Op == "+" && x.Type == "ptr" {
					// This is a string concatenation (dest type is ptr/string)

					// Helper to get type
					getType := func(val hir.Value) types.T {
						if v, ok := val.(hir.Var); ok {
							return m.varTypes[v.Name]
						}
						if t, ok := val.(hir.Temp); ok {
							name := t.Name
							if strings.HasPrefix(name, "%") {
								name = name[1:]
							}
							return m.varTypes[name]
						}
						return nil
					}

					lType := getType(x.LHS)
					rType := getType(x.RHS)

					leftStr := lval
					rightStr := rval

					// Convert left if it's not already a string (ptr)
					if lty != "ptr" {
						convTemp := fmt.Sprintf("%%str_conv_%d", m.tempID)
						m.tempID++
						switch lty {
						case "i32":
							wprintf(&m.funcs, "  %s = call ptr @int_to_str(i32 %s)\n", convTemp, lval)
							m.ensureDecl("declare ptr @int_to_str(i32)")
							leftStr = convTemp
						case "double", "float":
							wprintf(&m.funcs, "  %s = call ptr @float_to_str(double %s)\n", convTemp, lval)
							m.ensureDecl("declare ptr @float_to_str(double)")
							leftStr = convTemp
						case "i1":
							wprintf(&m.funcs, "  %s = call ptr @bool_to_str(i1 %s)\n", convTemp, lval)
							m.ensureDecl("declare ptr @bool_to_str(i1)")
							leftStr = convTemp
						}
					} else {
						// It's a ptr. Is it a string?
						isStr := false
						if lType != nil && types.Equal(lType, types.Str) {
							isStr = true
						} else if _, ok := x.LHS.(hir.ConstStr); ok {
							isStr = true
						}

						if !isStr {
							// Assume struct
							typeName := "Unknown"
							if lType != nil {
								if s, ok := lType.(*types.Struct); ok {
									typeName = s.Name
								}
							}
							convTemp := fmt.Sprintf("%%str_conv_%d", m.tempID)
							m.tempID++
							wprintf(&m.funcs, "  %s = call ptr @%s_to_str(ptr %s)\n", convTemp, typeName, lval)
							leftStr = convTemp
						}
					}

					// Convert right if it's not already a string (ptr)
					if rty != "ptr" {
						convTemp := fmt.Sprintf("%%str_conv_%d", m.tempID)
						m.tempID++
						switch rty {
						case "i32":
							wprintf(&m.funcs, "  %s = call ptr @int_to_str(i32 %s)\n", convTemp, rval)
							m.ensureDecl("declare ptr @int_to_str(i32)")
							rightStr = convTemp
						case "double", "float":
							wprintf(&m.funcs, "  %s = call ptr @float_to_str(double %s)\n", convTemp, rval)
							m.ensureDecl("declare ptr @float_to_str(double)")
							rightStr = convTemp
						case "i1":
							wprintf(&m.funcs, "  %s = call ptr @bool_to_str(i1 %s)\n", convTemp, rval)
							m.ensureDecl("declare ptr @bool_to_str(i1)")
							rightStr = convTemp
						}
					} else {
						// It's a ptr. Is it a string?
						isStr := false
						if rType != nil && types.Equal(rType, types.Str) {
							isStr = true
						} else if _, ok := x.RHS.(hir.ConstStr); ok {
							isStr = true
						}

						if !isStr {
							// Assume struct
							typeName := "Unknown"
							if rType != nil {
								if s, ok := rType.(*types.Struct); ok {
									typeName = s.Name
								}
							}
							convTemp := fmt.Sprintf("%%str_conv_%d", m.tempID)
							m.tempID++
							wprintf(&m.funcs, "  %s = call ptr @%s_to_str(ptr %s)\n", convTemp, typeName, rval)
							rightStr = convTemp
						}
					}

					// Now concatenate
					wprintf(&m.funcs, "  %s = call ptr @string_concat(ptr %s, ptr %s)\n",
						x.Dst.String(), leftStr, rightStr)
					m.ssa[x.Dst.Name] = x.Dst
					m.tempTypes[x.Dst.Name] = "ptr" // Track that result is a string (ptr)

					dstName := x.Dst.Name
					if strings.HasPrefix(dstName, "%") {
						dstName = dstName[1:]
					}
					m.varTypes[dstName] = types.Str // Track high-level type
					m.ensureDecl("declare ptr @string_concat(ptr, ptr)")
					continue
				}

				// Map Desi operators to LLVM instructions
				// Check for float types
				isFloat := lty == "float" || lty == "double"

				var llvmInst string
				switch x.Op {
				case "+":
					if isFloat {
						llvmInst = "fadd"
					} else {
						llvmInst = "add"
					}
				case "-":
					if isFloat {
						llvmInst = "fsub"
					} else {
						llvmInst = "sub"
					}
				case "*":
					if isFloat {
						llvmInst = "fmul"
					} else {
						llvmInst = "mul"
					}
				case "/":
					if isFloat {
						llvmInst = "fdiv"
					} else {
						llvmInst = "sdiv"
					}
				case "%":
					if isFloat {
						llvmInst = "frem"
					} else {
						llvmInst = "srem"
					}
				case "==":
					if isFloat {
						llvmInst = "fcmp oeq"
					} else {
						llvmInst = "icmp eq"
					}
				case "!=":
					if isFloat {
						llvmInst = "fcmp one"
					} else {
						llvmInst = "icmp ne"
					}
				case "<":
					if isFloat {
						llvmInst = "fcmp olt"
					} else {
						llvmInst = "icmp slt"
					}
				case "<=":
					if isFloat {
						llvmInst = "fcmp ole"
					} else {
						llvmInst = "icmp sle"
					}
				case ">":
					if isFloat {
						llvmInst = "fcmp ogt"
					} else {
						llvmInst = "icmp sgt"
					}
				case ">=":
					if isFloat {
						llvmInst = "fcmp oge"
					} else {
						llvmInst = "icmp sge"
					}
				case "**":
					// Power operator - not a native LLVM instruction
					// For now, emit a call to a runtime function
					wprintf(&m.funcs, "  %s = call i64 @__pow_i64(%s, %s)\n",
						x.Dst.String(), lval, rval)
					m.ensureDecl("declare i64 @__pow_i64(i64, i64)")
					m.tempTypes[x.Dst.Name] = x.Type
					continue
				case "and":
					llvmInst = "and"
				case "or":
					llvmInst = "or"
				default:
					// Unknown operator
					wprintf(&m.funcs, "  ; Unknown operator: %s\n", x.Op)
					continue
				}

				// Emit the operation (rty is same as lty for now, use lty)
				_ = rty // Suppress unused variable warning
				wprintf(&m.funcs, "  %s = %s %s %s, %s\n",
					x.Dst.String(), llvmInst, lty, lval, rval)
				m.tempTypes[x.Dst.Name] = x.Type

			case *hir.Ret:
				// Emit lifetime.end for all locals *before* the ret (once).
				if !lifetimesClosed {
					for i := len(locals) - 1; i >= 0; i-- {
						li := locals[i]
						wprintf(&m.funcs, "%s", intrin.LifetimeEnd(li.size, li.name))
					}
					lifetimesClosed = true
				}
				m.emitRet(x)

			case *hir.Drop:
				m.emitDrop(x)
			case *hir.IncRef:
				// Tier-0 no-op
			case *hir.DecRef:
				if v, ok := x.Val.(hir.Var); ok {
					wprintf(&m.funcs, "  call void @__rc_dec(ptr %%%s)\n", v.Name)
					m.needRcDec = true
				}

			// ------- M8H frame sugar (SSA-only aliases) -------
			case *hir.FrameSet:
				m.ssa[x.Slot] = x.Val
			case *hir.FrameGet:
				if v, ok := m.ssa[x.Slot]; ok {
					m.ssa[x.Dst.Name] = v
				} else {
					m.ssa[x.Dst.Name] = hir.ConstInt{Text: "0"}
				}

			// ------- arena helpers -------
			case *hir.ArenaAlloc:
				if v, ok := x.Arena.(hir.Var); ok {
					wprintf(&m.funcs, "  %%t%d = call ptr @__arena_alloc(ptr %%%s)\n", m.tempID, v.Name)
					m.ensureDecl("declare ptr @__arena_alloc(ptr)")
					m.tempID++
				}
			case *hir.DestroyArena:
				if v, ok := x.Arena.(hir.Var); ok {
					wprintf(&m.funcs, "  call void @__arena_destroy(ptr %%%s)\n", v.Name)
					m.needArena = true
				}

			// ------- tuple ops -------
			case *hir.InsertValue:
				m.emitInsertValue(x)
			case *hir.ExtractValue:
				m.emitExtractValue(x)

			// ------- M14: Low-level memory ops -------
			case *hir.Alloca:
				ty := x.Type
				if ty == "" {
					ty = "i32"
				}
				count := x.Count
				if count < 1 {
					count = 1
				}
				if count > 1 {
					wprintf(&m.funcs, "  %s = alloca %s, i32 %d\n", x.Dst.Name, ty, count)
				} else {
					wprintf(&m.funcs, "  %s = alloca %s\n", x.Dst.Name, ty)
				}
				m.tempTypes[x.Dst.Name] = "ptr"

			case *hir.Store:
				// Emit store instruction: store <type> <value>, <type>* <pointer>
				dstOp := m.ptrOperand(x.Dst)
				valTy, valOp := m.operand(x.Val)
				// Auto-register static field globals (globals starting with @)
				if v, ok := x.Dst.(hir.Var); ok && strings.HasPrefix(v.Name, "@") {
					if _, exists := m.staticFieldGlobals[v.Name]; !exists {
						m.staticFieldGlobals[v.Name] = valTy
					}
				}
				wprintf(&m.funcs, "  store %s %s, %s\n", valTy, valOp, dstOp)

			case *hir.Load:
				ptrOp := m.ptrOperand(x.Src)
				// Auto-register static field globals (globals starting with @)
				if v, ok := x.Src.(hir.Var); ok && strings.HasPrefix(v.Name, "@") {
					if _, exists := m.staticFieldGlobals[v.Name]; !exists {
						m.staticFieldGlobals[v.Name] = x.Type
					}
				}
				wprintf(&m.funcs, "  %s = load %s, %s\n", x.Dst.Name, x.Type, ptrOp)
				m.tempTypes[x.Dst.Name] = x.Type
				if x.DesiType != nil {
					if t, ok := x.DesiType.(types.T); ok {
						m.varTypes[x.Dst.Name] = t
					}
				}

			case *hir.GetElementPtr:
				baseOp := m.ptrOperand(x.Base)
				var idxStr strings.Builder
				for i, idx := range x.Indices {
					if i > 0 {
						idxStr.WriteString(", ")
					}
					idxStr.WriteString(m.i32Operand(idx))
				}
				wprintf(&m.funcs, "  %s = getelementptr inbounds %s, %s, %s\n", x.Dst.Name, x.Type, baseOp, idxStr.String())
				m.tempTypes[x.Dst.Name] = "ptr"

			case *hir.BitCast:
				valTy, valOp := m.operand(x.Val)
				wprintf(&m.funcs, "  %s = bitcast %s %s to %s\n", x.Dst.Name, valTy, valOp, x.Type)
				m.tempTypes[x.Dst.Name] = x.Type

			case *hir.Cast:
				valTy, valOp := m.operand(x.Src)
				opcode := "bitcast"
				// Simple heuristic for Tier-0
				if (valTy == "i64" || valTy == "i32") && x.Type == "ptr" {
					opcode = "inttoptr"
				} else if valTy == "ptr" && (x.Type == "i64" || x.Type == "i32") {
					opcode = "ptrtoint"
				} else if valTy == "i64" && x.Type == "i32" {
					opcode = "trunc"
				} else if valTy == "i32" && x.Type == "i64" {
					opcode = "sext" // Assume signed integers for now
				} else if valTy == "i1" && (x.Type == "i64" || x.Type == "i32") {
					opcode = "zext"
				}
				wprintf(&m.funcs, "  %s = %s %s %s to %s\n", x.Dst.Name, opcode, valTy, valOp, x.Type)
				m.tempTypes[x.Dst.Name] = x.Type

			// ------- control flow (elided) -------
			case *hir.If:
				// Emit proper LLVM control flow for If statement
				cty, cval := m.operand(x.Cond)

				// Generate merge block label
				mergeLabel := fmt.Sprintf("merge%d", m.mergeID)
				m.mergeID++

				// Get unique labels for then/else blocks
				thenLabel := blockLabels[x.Then]
				elseLabel := mergeLabel
				if x.Else != nil {
					elseLabel = blockLabels[x.Else]
				}

				// Emit conditional branch
				wprintf(&m.funcs, "  br %s %s, label %%%s, label %%%s\n",
					cty, cval, thenLabel, elseLabel)

				// Mark that then/else blocks need terminator to merge
				m.cfBlocks[thenLabel] = mergeLabel
				if x.Else != nil {
					m.cfBlocks[elseLabel] = mergeLabel
				}

				// Emit merge label immediately to split the current block
				// Subsequent statements in this loop will be emitted into the merge block
				wprintf(&m.funcs, "%s:\n", mergeLabel)

			case *hir.While:
				// Emit proper LLVM loop structure:
				// entry:
				//   br label %loop_cond
				// loop_cond:
				//   <evaluate condition>
				//   br i1 %cond, label %loop_body, label %loop_exit
				// loop_body:
				//   <body statements>
				//   br label %loop_cond
				// loop_exit:
				//   <continue>

				// Get unique labels
				condLabel := blockLabels[x.CondBlock]
				bodyLabel := blockLabels[x.Body]
				exitLabel := fmt.Sprintf("loop_exit%d", m.mergeID)
				m.mergeID++

				// Emit unconditional branch to loop header
				wprintf(&m.funcs, "  br label %%%s\n", condLabel)

				// Store condition value and target labels for condition block
				m.cfLoopConds[condLabel] = x.Cond
				m.cfBlocks[condLabel] = exitLabel + "|" + bodyLabel

				// Mark body block to branch back to condition
				m.cfBlocks[bodyLabel] = condLabel

				// Emit exit label immediately to split the block
				wprintf(&m.funcs, "%s:\n", exitLabel)

			// ------- M8 async/futures -------
			case *hir.FutureNew:
				wprintf(&m.funcs, "  %s = call ptr @__future_new()\n", x.Dst.String())
				m.ensureDecl("declare ptr @__future_new()")
			case *hir.Await:
				wprintf(&m.funcs, "  %s = call i32 @__await_blocking(%s)\n", x.Dst.String(), m.ptrOperand(x.Fut))
				m.ensureDecl("declare i32 @__await_blocking(ptr)")
			case *hir.FutureComplete:
				wprintf(&m.funcs, "  call void @__future_complete(%s, %s)\n", m.ptrOperand(x.Fut), m.i32Operand(x.Val))
				m.ensureDecl("declare void @__future_complete(ptr, i32)")
			}
		}

		// Add terminator for control flow blocks (branches to merge)
		if mergeLabel, ok := m.cfBlocks[label]; ok {
			// Check if block already has a terminator (e.g., ret)
			hasTerminator := false
			if len(b.Stmts) > 0 {
				if _, isRet := b.Stmts[len(b.Stmts)-1].(*hir.Ret); isRet {
					hasTerminator = true
				}
			}
			if !hasTerminator {
				// Check if this is a loop condition block
				if condVal, isLoop := m.cfLoopConds[label]; isLoop {
					// Loop condition: emit conditional branch
					// Format: "exit_label|body_label"
					parts := strings.Split(mergeLabel, "|")
					exitLabel := parts[0]
					bodyLabel := parts[1]

					// Get condition value
					cty, cval := m.operand(condVal)
					wprintf(&m.funcs, "  br %s %s, label %%%s, label %%%s\n",
						cty, cval, bodyLabel, exitLabel)

					// Clean up loop condition map
					delete(m.cfLoopConds, label)
				} else {
					// Regular control flow block: unconditional branch
					wprintf(&m.funcs, "  br label %%%s\n", mergeLabel)
				}
			}
			// Remove from map
			delete(m.cfBlocks, label)

			// Do NOT emit merge label here - it was already emitted in the If/While case
		}

		// Close lifetimes for locals at end of block if we didn't just emit an early 'ret'.
		if !lifetimesClosed {
			for i := len(locals) - 1; i >= 0; i-- {
				li := locals[i]
				wprintf(&m.funcs, "%s", intrin.LifetimeEnd(li.size, li.name))
			}
		}
	}
	m.curFuncRetTy = ""
	wprintf(&m.funcs, "}\n")
}

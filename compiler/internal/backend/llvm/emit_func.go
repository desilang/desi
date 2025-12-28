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
	// Skip if already emitted (prevent duplicates like __top__ from imports)
	if m.emittedFunctions[fn.Name] {
		return
	}
	// Mark this function as emitted
	m.emittedFunctions[fn.Name] = true
	// Also mark as defined (for call emit to avoid extern declares)
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

				// If type is a reference type (set, dict, list, str, class) with an initializer,
				// use SSA directly without alloca (immutable variables / class instances)
				if x.Init != nil && isReferenceType(x.Type) {
					m.ssa[x.Name] = x.Init
					// Skip alloca for reference types and classes - they're already pointers
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
						// Check the actual type - classes are pointers
						if x.Type != nil {
							if _, ok := x.Type.(*types.Class); ok {
								llvmTy, size = "ptr", 8
							} else if isReferenceType(x.Type) {
								llvmTy, size = "ptr", 8
							} else {
								llvmTy, size = "i32", 4
							}
						} else {
							// No type info, default to i32
							llvmTy, size = "i32", 4
						}
					}
					// Even with initializer, record SSA alias for convenience.
					m.ssa[x.Name] = x.Init
				} else if !isReferenceType(x.Type) {
					// Value types without initializer
					llvmTy, size = "i32", 4
				}
				uniqueN := m.uniqueName(x.Name)
				wprintf(&m.funcs, "  %%%s = alloca %s\n", uniqueN, llvmTy)
				wprintf(&m.funcs, "%s", intrin.LifetimeStart(size, uniqueN))
				locals = append(locals, localInfo{name: uniqueN, size: size})

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
							// Assume struct or class
							typeName := "Unknown"
							if lType != nil {
								if s, ok := lType.(*types.Struct); ok {
									typeName = s.Name
								} else if c, ok := lType.(*types.Class); ok {
									typeName = c.Name
								}
							}
							// If still Unknown, check if it might be a string from a field load
							if typeName == "Unknown" {
								// Field loads from class methods often don't have type info
								// Just use the value directly as it's likely a string
								leftStr = lval
							} else {
								convTemp := fmt.Sprintf("%%str_conv_%d", m.tempID)
								m.tempID++
								wprintf(&m.funcs, "  %s = call ptr @%s_to_str(ptr %s)\n", convTemp, typeName, lval)
								leftStr = convTemp
							}
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
							// Assume struct or class
							typeName := "Unknown"
							if rType != nil {
								if s, ok := rType.(*types.Struct); ok {
									typeName = s.Name
								} else if c, ok := rType.(*types.Class); ok {
									typeName = c.Name
								}
							}
							// If still Unknown, check if it might be a string from a field load
							if typeName == "Unknown" {
								// Field loads from class methods often don't have type info
								// Just use the value directly as it's likely a string
								rightStr = rval
							} else {
								convTemp := fmt.Sprintf("%%str_conv_%d", m.tempID)
								m.tempID++
								wprintf(&m.funcs, "  %s = call ptr @%s_to_str(ptr %s)\n", convTemp, typeName, rval)
								rightStr = convTemp
							}
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
				case "^":
					llvmInst = "xor"
				case "|":
					llvmInst = "or"
				case "&":
					llvmInst = "and"
				case "<<":
					llvmInst = "shl"
				case ">>":
					llvmInst = "ashr" // Arithmetic shift right for signed integers
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
				// Get arena operand - use m.operand to resolve SSA aliases
				_, arenaOp := m.operand(x.Arena)

				// Get size operand from Args[0], default to 8 bytes if not provided
				sizeOp := "8"
				if len(x.Args) > 0 {
					_, sizeOp = m.operand(x.Args[0])
				}

				// Emit call and store result to destination temp
				wprintf(&m.funcs, "  %s = call ptr @__arena_alloc(ptr %s, i64 %s)\n", x.Dst.String(), arenaOp, sizeOp)
				m.ensureDecl("declare ptr @__arena_alloc(ptr, i64)")
				m.tempTypes[x.Dst.Name] = "ptr"
			case *hir.DestroyArena:
				// Use ptrOperand to resolve SSA aliases
				arenaPtr := m.ptrOperand(x.Arena)
				wprintf(&m.funcs, "  call void @__arena_destroy(%s)\n", arenaPtr)
				m.needArena = true

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

				// Get bit widths for integer types
				srcBits := intTypeBits(valTy)
				dstBits := intTypeBits(x.Type)

				// Check if types are floats
				srcFloat := (valTy == "float" || valTy == "double")
				dstFloat := (x.Type == "float" || x.Type == "double")

				// Handle all cast combinations
				if srcBits > 0 && dstBits > 0 {
					// Integer to integer
					if srcBits > dstBits {
						opcode = "trunc"
					} else if srcBits < dstBits {
						opcode = "sext" // Use sext for signed integers
					} // else same size, use bitcast (though unusual)
				} else if srcFloat && dstFloat {
					// Float to float
					if valTy == "double" && x.Type == "float" {
						opcode = "fptrunc"
					} else if valTy == "float" && x.Type == "double" {
						opcode = "fpext"
					}
				} else if srcBits > 0 && dstFloat {
					// Integer to float
					opcode = "sitofp"
				} else if srcFloat && dstBits > 0 {
					// Float to integer
					opcode = "fptosi"
				} else if (valTy == "i64" || valTy == "i32") && x.Type == "ptr" {
					opcode = "inttoptr"
				} else if valTy == "ptr" && (x.Type == "i64" || x.Type == "i32") {
					opcode = "ptrtoint"
				} else if valTy == "i1" && (x.Type == "i64" || x.Type == "i32" || x.Type == "i16" || x.Type == "i8") {
					opcode = "zext"
				} else if valTy == "i1" && x.Type == "ptr" {
					// Special case: bool to ptr requires two steps: i1 -> i64 -> ptr
					tmpName := x.Dst.Name + "_ext"
					wprintf(&m.funcs, "  %s = zext i1 %s to i64\n", tmpName, valOp)
					wprintf(&m.funcs, "  %s = inttoptr i64 %s to ptr\n", x.Dst.Name, tmpName)
					m.tempTypes[x.Dst.Name] = x.Type
					continue // Skip normal emit below
				} else if (valTy == "double" || valTy == "float") && x.Type == "ptr" {
					// Special case: float/double to ptr requires boxing (can't bitcast float to ptr)
					// Allocate memory for the float value and store it
					boxSize := "8" // double is 8 bytes
					if valTy == "float" {
						boxSize = "4"
					}
					boxPtr := x.Dst.Name + "_box"
					m.ensureDecl("declare ptr @malloc(...)")
					wprintf(&m.funcs, "  %s = call ptr @malloc(i64 %s)\n", boxPtr, boxSize)
					wprintf(&m.funcs, "  store %s %s, ptr %s\n", valTy, valOp, boxPtr)
					// The destination is the box pointer
					wprintf(&m.funcs, "  %s = bitcast ptr %s to ptr\n", x.Dst.Name, boxPtr)
					m.tempTypes[x.Dst.Name] = "ptr"
					continue // Skip normal emit below
				} else if valTy == "ptr" && (x.Type == "double" || x.Type == "float") {
					// Special case: ptr to float/double requires unboxing (load from boxed pointer)
					wprintf(&m.funcs, "  %s = load %s, ptr %s\n", x.Dst.Name, x.Type, valOp)
					m.tempTypes[x.Dst.Name] = x.Type
					continue // Skip normal emit below
				} else if valTy == "ptr" && x.Type == "i1" {
					// Special case: ptr to bool requires ptrtoint then trunc
					tmpName := x.Dst.Name + "_i64"
					wprintf(&m.funcs, "  %s = ptrtoint ptr %s to i64\n", tmpName, valOp)
					wprintf(&m.funcs, "  %s = trunc i64 %s to i1\n", x.Dst.Name, tmpName)
					m.tempTypes[x.Dst.Name] = "i1"
					continue // Skip normal emit below
				}
				wprintf(&m.funcs, "  %s = %s %s %s to %s\n", x.Dst.Name, opcode, valTy, valOp, x.Type)
				m.tempTypes[x.Dst.Name] = x.Type

			case *hir.Select:
				// Emit LLVM select instruction: %dst = select i1 %cond, type %then, type %else
				_, condVal := m.operand(x.Cond)
				thenTy, thenVal := m.operand(x.Then)
				_, elseVal := m.operand(x.Else)
				wprintf(&m.funcs, "  %s = select i1 %s, %s %s, %s %s\n",
					x.Dst.Name, condVal, thenTy, thenVal, thenTy, elseVal)
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

			// ------- Drop (RAII cleanup) -------
			case *hir.Drop:
				// Resolve variable through SSA map if it's aliased
				var actualVal hir.Value = x.Val
				needsLoad := false
				if v, ok := x.Val.(hir.Var); ok {
					if alias, exists := m.ssa[v.Name]; exists {
						actualVal = alias
					} else {
						// No SSA alias means variable uses alloca - need to load before free
						needsLoad = true
					}
				}
				valOp := m.ptrOperand(actualVal)

				// If variable is stored via alloca, load the actual heap pointer first
				if needsLoad {
					loadTemp := fmt.Sprintf("%%drop_load_%d", m.tempID)
					m.tempID++
					// valOp already contains "ptr %varname", extract just the %varname part
					wprintf(&m.funcs, "  %s = load ptr, %s\n", loadTemp, valOp)
					valOp = "ptr " + loadTemp
				}

				if x.Type != nil {
					if classType, ok := x.Type.(*types.Class); ok {
						// Emit __del__ calls for class and all base classes (child → parent order)
						m.emitDestructorChain(classType, valOp)
						// Free the class instance after destructor
						wprintf(&m.funcs, "  call void @free(%s)\n", valOp)
						m.ensureDecl("declare void @free(ptr)")
					}
				}

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
					lifetimesClosed = true // Don't emit lifetime.end after branch
				} else {
					// Regular control flow block: unconditional branch
					wprintf(&m.funcs, "  br label %%%s\n", mergeLabel)
					lifetimesClosed = true // Don't emit lifetime.end after branch
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

// intTypeBits returns the bit width for LLVM integer types, or 0 if not an integer
func intTypeBits(ty string) int {
	switch ty {
	case "i1":
		return 1
	case "i8":
		return 8
	case "i16":
		return 16
	case "i32":
		return 32
	case "i64":
		return 64
	case "i128":
		return 128
	default:
		return 0
	}
}

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
	m.localRegs = make(map[string]string)
	m.curRetIsPtr = m.asyncWrappers[fn.Name]
	m.currentMoves = nil
	m.curFuncName = fn.Name // Track for call depth instrumentation

	// Record where this function's text begins so we can hoist its allocas
	// to the entry block at the end (see hoistAllocasToEntry). EmitFunc is
	// not reentrant into m.funcs, so the function's text is the buffer tail
	// [funcStart:] once emission completes.
	funcStart := m.funcs.Len()

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
	// Add inline attribute if requested
	attrs := ""
	if fn.Inline {
		attrs = " alwaysinline"
	}
	// Functions containing setjmp (try/except) must keep a real frame
	// pointer: llvm.frameaddress(0) feeds _setjmp/RtlUnwindEx on Windows,
	// and -O2's frame-pointer omission would hand longjmp a garbage frame
	// (access violation during unwind). noinline keeps the setjmp frame in
	// the function that owns the jmp_buf.
	hasSetjmp := false
setjmpScan:
	for _, b := range fn.Blocks {
		for _, s := range b.Stmts {
			if c, ok := s.(*hir.Call); ok && c.Fn == "setjmp" {
				hasSetjmp = true
				break setjmpScan
			}
		}
	}
	if hasSetjmp {
		attrs += " noinline \"frame-pointer\"=\"all\""
	}
	wprintf(&m.funcs, ")%s {\n", attrs)

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

	isMainFunc := fn.Name == "main"
	firstBlock := true
	for _, b := range fn.Blocks {
		label := blockLabels[b]
		wprintf(&m.funcs, "%s:\n", label)

		// Initialize runtime at start of main function
		if isMainFunc && firstBlock {
			m.ensureDecl("declare void @__desi_runtime_init()")
			wprintf(&m.funcs, "  call void @__desi_runtime_init()\n")

			// Call __top__ to initialize global variables (only if defined)
			if m.definedFunctions["__top__"] {
				wprintf(&m.funcs, "  call i32 @__top__()\n")
			}
			firstBlock = false
		} else if firstBlock && fn.Name != "main" && !strings.HasSuffix(fn.Name, "__top__") {
			// Emit call depth tracking for user functions (skip main and __top__)
			m.ensureDecl("declare void @__desi_call_enter(ptr)")
			// Create a string constant for the function name (strip internal prefixes)
			displayName := strings.TrimPrefix(fn.Name, "__desi$")
			fnNameStr, fnNameLen := m.ensureCStringGlobal(displayName, false)
			fnNameGEP := fmt.Sprintf("getelementptr inbounds ([%d x i8], [%d x i8]* %s, i64 0, i64 0)", fnNameLen, fnNameLen, fnNameStr)
			wprintf(&m.funcs, "  call void @__desi_call_enter(ptr %s)\n", fnNameGEP)
			firstBlock = false
		}

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
				if x.Type != nil {
					if typ, ok := x.Type.(types.T); ok {
						tyStr := typ.String()
						switch tyStr {
						case "float", "f64":
							llvmTy, size = "double", 8
						case "f32":
							llvmTy, size = "float", 4
						case "bool":
							llvmTy, size = "i1", 1
						case "i64", "u64", "usize", "isize":
							llvmTy, size = "i64", 8
						case "i16", "u16":
							llvmTy, size = "i16", 2
						case "i8", "u8", "byte":
							llvmTy, size = "i8", 1
						case "i128", "u128":
							llvmTy, size = "i128", 16
						case "int", "i32", "u32":
							llvmTy, size = "i32", 4
						default:
							if _, ok := typ.(*types.Class); ok {
								llvmTy, size = "ptr", 8
							} else if isReferenceType(typ) {
								llvmTy, size = "ptr", 8
							} else {
								llvmTy, size = "i32", 4
							}
						}
					} else {
						llvmTy, size = "i32", 4
					}
					if x.Init != nil {
						m.ssa[x.Name] = x.Init
					}
				} else if x.Init != nil {
					switch initVal := x.Init.(type) {
					case hir.ConstBool:
						llvmTy, size = "i1", 1
					case hir.ConstStr:
						llvmTy, size = "ptr", 8
					case hir.ConstInt:
						llvmTy, size = "i32", 4
					case hir.ConstFloat:
						llvmTy, size = "double", 8
					case hir.Temp:
						if ty, exists := m.tempTypes[initVal.Name]; exists {
							llvmTy = ty
							if ty == "ptr" || ty == "double" {
								size = 8
							} else if ty == "i1" || ty == "i8" {
								size = 1
							} else if ty == "i16" {
								size = 2
							} else {
								size = 4
							}
						} else {
							llvmTy, size = "i32", 4
						}
					}
					m.ssa[x.Name] = x.Init
				} else if !isReferenceType(x.Type) {
					// Value types without initializer
					llvmTy, size = "i32", 4
				}
				uniqueN := m.uniqueName(x.Name)
				m.declareLocal(x.Name, uniqueN)
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
						case "i32", "i64", "i8", "i16":
							extVal := lval
							if lty != "i64" {
								extVal = fmt.Sprintf("%%iext_%d", m.tempID)
								m.tempID++
								wprintf(&m.funcs, "  %s = sext %s %s to i64\n", extVal, lty, lval)
							}
							wprintf(&m.funcs, "  %s = call ptr @int_to_str(i64 %s)\n", convTemp, extVal)
							m.ensureDecl("declare ptr @int_to_str(i64)")
							leftStr = convTemp
						case "double", "float":
							wprintf(&m.funcs, "  %s = call ptr @float_to_str(double %s)\n", convTemp, lval)
							m.ensureDecl("declare ptr @float_to_str(double)")
							leftStr = convTemp
						case "i1":
							convExt := fmt.Sprintf("%%bext_%d", m.tempID)
							m.tempID++
							wprintf(&m.funcs, "  %s = zext i1 %s to i32\n", convExt, lval)
							wprintf(&m.funcs, "  %s = call ptr @bool_to_str(i32 %s)\n", convTemp, convExt)
							m.ensureDecl("declare ptr @bool_to_str(i32)")
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
						case "i32", "i64", "i8", "i16":
							extVal := rval
							if rty != "i64" {
								extVal = fmt.Sprintf("%%iext_%d", m.tempID)
								m.tempID++
								wprintf(&m.funcs, "  %s = sext %s %s to i64\n", extVal, rty, rval)
							}
							wprintf(&m.funcs, "  %s = call ptr @int_to_str(i64 %s)\n", convTemp, extVal)
							m.ensureDecl("declare ptr @int_to_str(i64)")
							rightStr = convTemp
						case "double", "float":
							wprintf(&m.funcs, "  %s = call ptr @float_to_str(double %s)\n", convTemp, rval)
							m.ensureDecl("declare ptr @float_to_str(double)")
							rightStr = convTemp
						case "i1":
							convExt := fmt.Sprintf("%%bext_%d", m.tempID)
							m.tempID++
							wprintf(&m.funcs, "  %s = zext i1 %s to i32\n", convExt, rval)
							wprintf(&m.funcs, "  %s = call ptr @bool_to_str(i32 %s)\n", convTemp, convExt)
							m.ensureDecl("declare ptr @bool_to_str(i32)")
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
					// Division by zero check for both int and float
					m.ensureDecl("declare void @__panic_divzero()")
					checkLabel := fmt.Sprintf("divzero_check_%d", m.tempID)
					okLabel := fmt.Sprintf("divzero_ok_%d", m.tempID)
					m.tempID++

					if isFloat {
						// Float: use fcmp oeq for 0.0 comparison
						wprintf(&m.funcs, "  %%divzero_cmp_%d = fcmp oeq %s %s, 0.0\n", m.tempID, lty, rval)
						wprintf(&m.funcs, "  br i1 %%divzero_cmp_%d, label %%%s, label %%%s\n", m.tempID, checkLabel, okLabel)
						m.tempID++
						wprintf(&m.funcs, "\n%s:\n", checkLabel)
						wprintf(&m.funcs, "  call void @__panic_divzero()\n")
						wprintf(&m.funcs, "  unreachable\n")
						wprintf(&m.funcs, "\n%s:\n", okLabel)
						llvmInst = "fdiv"
					} else {
						// Integer: use icmp eq for 0 comparison
						wprintf(&m.funcs, "  %%divzero_cmp_%d = icmp eq %s %s, 0\n", m.tempID, lty, rval)
						wprintf(&m.funcs, "  br i1 %%divzero_cmp_%d, label %%%s, label %%%s\n", m.tempID, checkLabel, okLabel)
						m.tempID++
						wprintf(&m.funcs, "\n%s:\n", checkLabel)
						wprintf(&m.funcs, "  call void @__panic_divzero()\n")
						wprintf(&m.funcs, "  unreachable\n")
						wprintf(&m.funcs, "\n%s:\n", okLabel)
						llvmInst = "sdiv"
					}
				case "%":
					if isFloat {
						llvmInst = "frem"
					} else {
						// Integer modulo: also check for division by zero
						m.ensureDecl("declare void @__panic_divzero()")
						checkLabel := fmt.Sprintf("divzero_check_%d", m.tempID)
						okLabel := fmt.Sprintf("divzero_ok_%d", m.tempID)
						m.tempID++
						wprintf(&m.funcs, "  %%divzero_cmp_%d = icmp eq %s %s, 0\n", m.tempID, lty, rval)
						wprintf(&m.funcs, "  br i1 %%divzero_cmp_%d, label %%%s, label %%%s\n", m.tempID, checkLabel, okLabel)
						m.tempID++
						wprintf(&m.funcs, "\n%s:\n", checkLabel)
						wprintf(&m.funcs, "  call void @__panic_divzero()\n")
						wprintf(&m.funcs, "  unreachable\n")
						wprintf(&m.funcs, "\n%s:\n", okLabel)
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
				// Track the result type based on actual emitted instruction:
				// - Comparisons (icmp/fcmp) always produce i1
				// - Arithmetic ops produce the operand type (lty)
				if strings.HasPrefix(llvmInst, "icmp") || strings.HasPrefix(llvmInst, "fcmp") {
					m.tempTypes[x.Dst.Name] = "i1"
				} else {
					m.tempTypes[x.Dst.Name] = lty
				}

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

			case *hir.TailCall:
				// TCO: Emit tail call that LLVM can optimize
				// For self-recursive calls, we emit a tail call instruction
				// which LLVM will optimize if possible
				var argParts []string
				for i, arg := range x.Args {
					_, val := m.operand(arg)
					// Get param type from function signature
					paramType := "ptr" // default
					if i < len(fn.Params) {
						paramType = fn.Params[i].Type
					}
					argParts = append(argParts, fmt.Sprintf("%s %s", paramType, val))
				}
				args := strings.Join(argParts, ", ")

				// Build return type and function signature
				retType := "i32" // default
				if fn.RetType != "" && fn.RetType != "void" {
					retType = fn.RetType
				}

				// Decrement call depth BEFORE tail call to avoid false recursion limit
				// (since tail call is effectively a return-then-call, not a nested call)
				m.ensureDecl("declare void @__desi_call_exit()")
				wprintf(&m.funcs, "  call void @__desi_call_exit()\n")

				// Emit tail call
				if fn.RetType == "" || fn.RetType == "void" {
					wprintf(&m.funcs, "  tail call void @%s(%s)\n", x.Fn, args)
					wprintf(&m.funcs, "  ret void\n")
				} else {
					tailResult := fmt.Sprintf("%%tailcall_%d", m.tempID)
					m.tempID++
					wprintf(&m.funcs, "  %s = tail call %s @%s(%s)\n", tailResult, retType, x.Fn, args)
					wprintf(&m.funcs, "  ret %s %s\n", retType, tailResult)
				}

			case *hir.IncRef:
				// Tier-0 no-op
			case *hir.DecRef:
				if v, ok := x.Val.(hir.Var); ok {
					// Check if variable is in SSA map (aliased to temp)
					var valOp string
					if alias, exists := m.ssa[v.Name]; exists {
						valOp = m.ptrOperand(alias)
					} else {
						// Variable uses alloca - need to load the actual pointer first
						loadTemp := fmt.Sprintf("%%decref_load_%d", m.tempID)
						m.tempID++
						wprintf(&m.funcs, "  %s = load ptr, ptr %%%s\n", loadTemp, v.Name)
						valOp = "ptr " + loadTemp
					}
					wprintf(&m.funcs, "  call void @__rc_dec(%s)\n", valOp)
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
					// Multi-element buffers get align 16: the try/except
					// jmp_buf is one of these, and MSVC's _setjmp saves SSE
					// registers into it with aligned 16-byte stores — a
					// default (align 1) i8 buffer only works when stack
					// layout happens to align it; -O2's stack packing broke
					// that (access violation inside _setjmp).
					wprintf(&m.funcs, "  %s = alloca %s, i32 %d, align 16\n", x.Dst.Name, ty, count)
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
						val := "0"
						if valTy == "ptr" {
							val = "null"
						}
						m.staticFieldGlobals[v.Name] = GlobalDef{Type: valTy, Value: val}
					}
				}
				wprintf(&m.funcs, "  store %s %s, %s\n", valTy, valOp, dstOp)

			case *hir.Load:
				ptrOp := m.ptrOperand(x.Src)
				// Auto-register static field globals (globals starting with @)
				if v, ok := x.Src.(hir.Var); ok && strings.HasPrefix(v.Name, "@") {
					if _, exists := m.staticFieldGlobals[v.Name]; !exists {
						val := "0"
						if x.Type == "ptr" {
							val = "null"
						}
						m.staticFieldGlobals[v.Name] = GlobalDef{Type: x.Type, Value: val}
					}
				}
				wprintf(&m.funcs, "  %s = load %s, %s\n", x.Dst.Name, x.Type, ptrOp)
				m.tempTypes[x.Dst.Name] = x.Type
				if x.DesiType != nil {
					if t, ok := x.DesiType.(types.T); ok {
						m.varTypes[x.Dst.Name] = t
					}
				}

			case *hir.AddressOf:
				// Address-of operator: allocate on stack and return pointer
				// If Src is a Var, check if it already has an alloca (ptr type)
				// or if it's an SSA value like a function parameter
				if varSrc, ok := x.Src.(hir.Var); ok {
					varTy := m.tempTypes[varSrc.Name]
					if varTy == "" || varTy == "ptr" {
						// Variable already has an alloca, just copy the pointer
						wprintf(&m.funcs, "  %s = bitcast ptr %%%s to ptr\n", x.Dst.Name, varSrc.Name)
					} else {
						// SSA value (e.g. function parameter) — alloca + store to create memory location
						allocTy := varTy
						allocTemp := x.Dst.Name + ".addr"
						wprintf(&m.funcs, "  %s = alloca %s\n", allocTemp, allocTy)
						wprintf(&m.funcs, "  store %s %%%s, ptr %s\n", allocTy, varSrc.Name, allocTemp)
						wprintf(&m.funcs, "  %s = bitcast ptr %s to ptr\n", x.Dst.Name, allocTemp)
					}
				} else {
					// For non-variable sources, alloca and store
					allocTy := x.Type
					if allocTy == "" {
						allocTy = "ptr"
					}
					allocTemp := x.Dst.Name + ".addr"
					wprintf(&m.funcs, "  %s = alloca %s\n", allocTemp, allocTy)
					srcOp, _ := m.operand(x.Src)
					wprintf(&m.funcs, "  store %s %s, ptr %s\n", allocTy, srcOp, allocTemp)
					wprintf(&m.funcs, "  %s = bitcast ptr %s to ptr\n", x.Dst.Name, allocTemp)
				}
				m.tempTypes[x.Dst.Name] = "ptr"

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

				// Mark lifetimes as closed for THIS block to prevent unreachable
				// lifetime.end calls. The merge block is a new block with its own lifetime tracking.
				lifetimesClosed = true

			case *hir.Jump:
				// Unconditional branch to target block
				targetLabel := blockLabels[x.Target]
				wprintf(&m.funcs, "  br label %%%s\n", targetLabel)
				lifetimesClosed = true

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

				// The lowerer owns the exit block when the loop can be left by
				// 'break', so that the jump has a label to name. Without one,
				// synthesise a label and split the current block as before.
				exitLabel := ""
				exitIsOwned := x.Exit != nil
				if exitIsOwned {
					exitLabel = blockLabels[x.Exit]
				} else {
					exitLabel = fmt.Sprintf("loop_exit%d", m.mergeID)
					m.mergeID++
				}

				// Emit unconditional branch to loop header
				wprintf(&m.funcs, "  br label %%%s\n", condLabel)

				// Store condition value and target labels for condition block
				m.cfLoopConds[condLabel] = x.Cond
				m.cfBlocks[condLabel] = exitLabel + "|" + bodyLabel

				// The back-edge leaves from the latch when there is one — that
				// is where the scope drops and the 'for' index increment live,
				// and where 'continue' lands. Otherwise the body closes the loop.
				if x.Latch != nil {
					m.cfBlocks[blockLabels[x.Latch]] = condLabel
				} else {
					m.cfBlocks[bodyLabel] = condLabel
				}

				if !exitIsOwned {
					// Emit exit label immediately to split the block
					wprintf(&m.funcs, "%s:\n", exitLabel)
				}
				lifetimesClosed = true

			// ------- Drop (RAII cleanup) -------
			case *hir.Drop:
				// Check if variable was moved (returned, passed by ownership, etc.)
				// Moved variables are handled elsewhere - skip drop
				if v, ok := x.Val.(hir.Var); ok {
					if m.currentMoves != nil && m.currentMoves[v.Name] {
						continue // Skip drop for moved variable
					}
				}

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

				// If the variable is stored via alloca, the actual heap pointer
				// has to be loaded before it can be freed — but only where
				// something below actually frees it. Emitting the load up front
				// gave every non-heap local a dead load, reading 8 bytes out of
				// (for an int) a 4-byte slot.
				loaded := false
				ensureLoaded := func() string {
					if needsLoad && !loaded {
						loadTemp := fmt.Sprintf("%%drop_load_%d", m.tempID)
						m.tempID++
						// valOp already contains "ptr %varname", extract just the %varname part
						wprintf(&m.funcs, "  %s = load ptr, %s\n", loadTemp, valOp)
						valOp = "ptr " + loadTemp
						loaded = true
					}
					return valOp
				}

				if x.Type != nil {
					if classType, ok := x.Type.(*types.Class); ok {
						// Verify the value is actually a pointer before freeing
						// Some class-typed values might be primitive results (e.g., operator calls)
						llvmTy, _ := m.operand(actualVal)
						if llvmTy == "ptr" {
							op := ensureLoaded()
							// Emit __del__ calls for class and all base classes (child → parent order)
							m.emitDestructorChain(classType, op)
							// Free the class instance after destructor
							wprintf(&m.funcs, "  call void @free(%s)\n", op)
							m.ensureDecl("declare void @free(ptr)")
						}
					} else if t, ok := x.Type.(types.T); ok {
						// Heap types: free via the type-aware drop helpers.
						// Str is temp-only — string LOCALS (hir.Var) may alias
						// literals or another owner's storage, so they are never
						// freed here. String TEMPS (hir.Temp) are only registered
						// for drop by the lowerer when they are unconsumed rvalue
						// results of heap-producing ops (string_concat), which
						// makes them provably owned and safe to free.
						if t == types.Str {
							if _, isTemp := x.Val.(hir.Temp); isTemp {
								llvmTy, _ := m.operand(actualVal)
								if llvmTy == "ptr" {
									wprintf(&m.funcs, "  call void @free(%s)\n", ensureLoaded())
									m.ensureDecl("declare void @free(ptr)")
								}
							}
							continue
						}
						switch t.(type) {
						case *types.List, *types.Set, *types.Dict, *types.Enum, *types.Struct:
							llvmTy, _ := m.operand(actualVal)
							if llvmTy == "ptr" {
								// valOp is "ptr %name" — emitDropForType wants the bare operand
								bare := strings.TrimPrefix(ensureLoaded(), "ptr ")
								m.emitDropForType(bare, t)
							}
						}
					}
				}

			// ------- M8 async/futures -------
			case *hir.FutureNew:
				wprintf(&m.funcs, "  %s = call ptr @__future_new()\n", x.Dst.String())
				m.ensureDecl("declare ptr @__future_new()")
				m.tempTypes[strings.TrimPrefix(x.Dst.String(), "%")] = "ptr"
			case *hir.Await:
				rawDst := fmt.Sprintf("%%await_raw_%d", m.tempID)
				m.tempID++
				wprintf(&m.funcs, "  %s = call i64 @__await_blocking(%s)\n", rawDst, m.ptrOperand(x.Fut))
				m.ensureDecl("declare i64 @__await_blocking(ptr)")

				// Narrow i64 result to the expected type
				dstName := strings.TrimPrefix(x.Dst.String(), "%")
				switch x.ResultType {
				case "i32":
					wprintf(&m.funcs, "  %s = trunc i64 %s to i32\n", x.Dst.String(), rawDst)
					m.tempTypes[dstName] = "i32"
				case "ptr":
					wprintf(&m.funcs, "  %s = inttoptr i64 %s to ptr\n", x.Dst.String(), rawDst)
					m.tempTypes[dstName] = "ptr"
				case "i1":
					wprintf(&m.funcs, "  %s = trunc i64 %s to i1\n", x.Dst.String(), rawDst)
					m.tempTypes[dstName] = "i1"
				case "double":
					wprintf(&m.funcs, "  %s = bitcast i64 %s to double\n", x.Dst.String(), rawDst)
					m.tempTypes[dstName] = "double"
				default:
					// No narrowing — use i64 as-is (or default to i32 for backwards compat)
					wprintf(&m.funcs, "  %s = trunc i64 %s to i32\n", x.Dst.String(), rawDst)
					m.tempTypes[dstName] = "i32"
				}
			case *hir.FutureComplete:
				wprintf(&m.funcs, "  call void @__future_complete(%s, %s)\n", m.ptrOperand(x.Fut), m.i32Operand(x.Val))
				m.ensureDecl("declare void @__future_complete(ptr, i64)")
			case *hir.FutureSpawn:
				// Emit: call void @__future_spawn_N(ptr %fut, ptr @body, i64 %a0, ...)
				// ptrOperand includes the type prefix (e.g., "ptr %x")
				nArgs := len(x.Args)
				spawnFn := fmt.Sprintf("__future_spawn_%d", nArgs)
				// Build arg string — fut is ptr, body is ptr, args are i64
				argStr := fmt.Sprintf("%s, ptr @%s", m.ptrOperand(x.Fut), x.BodyFn)
				for _, a := range x.Args {
					// Each arg must be i64 for the C runtime. Widen from source type.
					aty, aval := m.operand(a)
					var i64val string
					switch aty {
					case "i64":
						i64val = aval
					case "i32":
						tmp := fmt.Sprintf("%%sext_%d", m.tempID)
						m.tempID++
						wprintf(&m.funcs, "  %s = sext i32 %s to i64\n", tmp, aval)
						i64val = tmp
					case "ptr":
						tmp := fmt.Sprintf("%%p2i_%d", m.tempID)
						m.tempID++
						wprintf(&m.funcs, "  %s = ptrtoint ptr %s to i64\n", tmp, aval)
						i64val = tmp
					default:
						// Fallback: just use the raw value (may fail for unusual types)
						i64val = aval
					}
					argStr += fmt.Sprintf(", i64 %s", i64val)
				}
				wprintf(&m.funcs, "  call void @%s(%s)\n", spawnFn, argStr)
				// Build declaration
				declParams := "ptr, ptr"
				for i := 0; i < nArgs; i++ {
					declParams += ", i64"
				}
				m.ensureDecl(fmt.Sprintf("declare void @%s(%s)", spawnFn, declParams))
			}
		}

		// Check if the last statement was hir.If and the merge block needs a terminator
		// This handles the case where both branches return, leaving the merge block empty
		// Only do this if the block is NOT in cfBlocks (so it won't get a branch added later)
		if len(b.Stmts) > 0 {
			if _, isIf := b.Stmts[len(b.Stmts)-1].(*hir.If); isIf {
				// The last statement was an If, meaning the merge block was just emitted
				// but has no content. We need to add a terminator.
				// Only do this if this block won't get a branch added by cfBlocks handling
				if lifetimesClosed && m.cfBlocks[label] == "" {
					wprintf(&m.funcs, "  unreachable\n")
				}
			}
		}

		// Add terminator for control flow blocks (branches to merge)
		if mergeLabel, ok := m.cfBlocks[label]; ok {
			// Check if block already has a terminator. A 'ret' ends the block, and
			// so does the jump a break or continue emits — appending the branch to
			// the merge label after either one leaves two terminators in the block.
			hasTerminator := false
			if len(b.Stmts) > 0 {
				switch b.Stmts[len(b.Stmts)-1].(type) {
				case *hir.Ret, *hir.Jump:
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
					// Regular control flow block: unconditional branch to merge
					wprintf(&m.funcs, "  br label %%%s\n", mergeLabel)
					lifetimesClosed = true // Don't emit lifetime.end after branch
				}
			}
			// Remove from cfBlocks map
			delete(m.cfBlocks, label)
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

	// Hoist every alloca in this function to the top of its entry block.
	// An alloca emitted inside a loop body (option/result method result
	// slots, match-expression slots, dict get/setdefault spills, ...) is
	// re-executed each iteration and only reclaimed at function return, so
	// a long loop overflows the stack (the f-string bug class, but general).
	// Alloca instructions have no SSA operands — only a compile-time type
	// and constant count — so moving the lines up is always dependency-safe,
	// and the entry block dominates every use. This is what C frontends do.
	if hoisted, changed := hoistAllocasToEntry(m.funcs.String()[funcStart:]); changed {
		m.funcs.Truncate(funcStart)
		m.funcs.WriteString(hoisted)
	}
}

// hoistAllocasToEntry moves every `%x = alloca ...` line in one function's
// IR text to just after its `entry:` label. Returns the rewritten text and
// whether anything moved. Safe because alloca lines reference no SSA values.
func hoistAllocasToEntry(fnText string) (string, bool) {
	lines := strings.Split(fnText, "\n")
	var allocas []string
	rest := make([]string, 0, len(lines))
	entryInsert := -1 // index into rest just after the entry label
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "%") && strings.Contains(t, " = alloca ") {
			allocas = append(allocas, ln)
			continue
		}
		rest = append(rest, ln)
		if entryInsert == -1 && t == "entry:" {
			entryInsert = len(rest)
		}
	}
	if len(allocas) == 0 || entryInsert == -1 {
		return fnText, false
	}
	out := make([]string, 0, len(rest)+len(allocas))
	out = append(out, rest[:entryInsert]...)
	out = append(out, allocas...)
	out = append(out, rest[entryInsert:]...)
	return strings.Join(out, "\n"), true
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

package llvm

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/backend/llvm/abi"
	"github.com/desilang/desi/compiler/internal/backend/llvm/intrin"
	"github.com/desilang/desi/compiler/internal/check"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/term"
	"github.com/desilang/desi/compiler/internal/types"
)

// wprintf writes formatted text while ignoring write errors.
func wprintf(w io.Writer, format string, a ...any) {
	term.Write(w, []byte(fmt.Sprintf(format, a...)))
}

type Module struct {
	name      string
	globals   bytes.Buffer
	funcs     bytes.Buffer
	strLits   map[string]string    // text-key -> global name
	strOrder  []string             // deterministic order
	ssa       map[string]hir.Value // SSA-alias for simple variables
	tempTypes map[string]string    // Map of temp/SSA names to their LLVM types (e.g. "%t1" -> "ptr")

	// Map of variable names to their high-level Desi types
	varTypes map[string]types.T

	// Current function being emitted
	curFunc          *hir.Func
	tempID           int             // counter for %t0, %t1, ...
	mergeID          int             // counter for merge blocks
	asyncWrappers    map[string]bool // functions returning ptr (future handle)
	definedFunctions map[string]bool // track which functions we've defined
	needPuts         bool
	needRcDec        bool
	needArena        bool
	curFuncRetTy     string
	curRetIsPtr      bool
	cfBlocks         map[string]string    // blocks that need terminators to merge labels
	cfLoopConds      map[string]hir.Value // loop condition blocks -> condition value
	wroteGlob        bool
	info             *check.Info     // type checker info for move analysis
	currentMoves     map[string]bool // moved variables in current function
}

func NewModule(name string) *Module {
	return &Module{
		name:             name,
		strLits:          make(map[string]string),
		asyncWrappers:    make(map[string]bool),
		definedFunctions: make(map[string]bool),
		cfBlocks:         make(map[string]string),
		cfLoopConds:      make(map[string]hir.Value),
	}
}

// MarkAsyncWrapper records that calls to 'name' return a ptr (future handle).
func (m *Module) MarkAsyncWrapper(name string) {
	m.asyncWrappers[name] = true
}

func (m *Module) nextStrName() string {
	return fmt.Sprintf("@.str.%d", len(m.strOrder))
}

// include the NUL in the count and key deterministically
func (m *Module) ensureCStringGlobal(text string, withNewline bool) (gname string, count int) {
	payload := text
	if withNewline && !strings.HasSuffix(payload, "\n") {
		payload += "\n"
	}
	count = len(payload) + 1 // +1 for NUL
	key := fmt.Sprintf("%d:%s", count, payload)
	if name, ok := m.strLits[key]; ok {
		return name, count
	}
	name := m.nextStrName()
	m.strLits[key] = name
	m.strOrder = append(m.strOrder, key)
	return name, count
}

func (m *Module) ensureDecl(line string) {
	if strings.Contains(m.globals.String(), line) {
		return
	}
	wprintf(&m.globals, "%s\n", line)
}

func (m *Module) writeGlobals() {
	if m.wroteGlob {
		return
	}
	m.wroteGlob = true

	type item struct{ key, name string }
	var items []item
	for i, key := range m.strOrder {
		name := m.strLits[key]
		items = append(items, item{key: fmt.Sprintf("%06d:%s", i, key), name: name})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].key < items[j].key })

	for _, it := range items {
		s := it.key[strings.Index(it.key, ":")+1:]
		parts := strings.SplitN(s, ":", 2)
		if len(parts) != 2 {
			continue
		}
		count, _ := strconv.Atoi(parts[0])
		payload := parts[1]
		escaped := escapeForCString(payload)
		wprintf(&m.globals, "%s = private unnamed_addr constant [%d x i8] c\"%s\\00\", align 1\n",
			it.name, count, escaped)
	}
	if m.needPuts {
		m.ensureDecl("declare i32 @puts(ptr)")
	}
	if m.needRcDec {
		m.ensureDecl("declare void @__rc_dec(ptr)")
	}
	if m.needArena {
		m.ensureDecl("declare void @__arena_destroy(ptr)")
	}
}

func (m *Module) IR() string {
	m.writeGlobals()
	var out bytes.Buffer
	// Use ABI layer for target information
	abiInfo := abi.Current()
	out.WriteString(fmt.Sprintf("target datalayout = \"%s\"\n", abiInfo.TargetLayout))
	out.WriteString(fmt.Sprintf("target triple = \"%s\"\n\n", abiInfo.TargetTriple))

	out.Write(m.globals.Bytes())
	out.WriteString("\n")
	out.Write(m.funcs.Bytes())
	return out.String()
}

func escapeForCString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\5C")
	s = strings.ReplaceAll(s, "\"", "\\22")
	s = strings.ReplaceAll(s, "\n", "\\0A")
	return s
}

// EmitFunc : Tier-0 subset—calls, returns, lifetimes for locals.
//   - Emit lifetime.end for block locals immediately *before* an unconditional 'ret'.
//   - Do NOT emit lifetime.end *after* the 'ret'.
//
// RegisterFunc marks a function as defined in this module.
func (m *Module) RegisterFunc(name string) {
	m.definedFunctions[name] = true
}

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

				// If type is a reference type (set, dict, list, str), don't allocate
				// Just use the init value directly as an SSA value
				if isReferenceType(x.Type) {
					if x.Init != nil {
						m.ssa[x.Name] = x.Init
					}
					// Skip alloca for reference types - they're already pointers
					continue
				}

				// For value types, emit alloca as before
				llvmTy, size := "i32", 4
				if x.Init != nil {
					switch x.Init.(type) {
					case hir.ConstBool:
						llvmTy, size = "i1", 1
					case hir.ConstStr:
						llvmTy, size = "ptr", 8
					case hir.ConstInt:
						llvmTy, size = "i32", 4
					case hir.Temp:
						llvmTy, size = "i32", 4
					}
					// Even with initializer, record SSA alias for convenience.
					m.ssa[x.Name] = x.Init
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
				wprintf(&m.funcs, "  store %s %s, %s\n", valTy, valOp, dstOp)

			case *hir.Load:
				ptrOp := m.ptrOperand(x.Src)
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

func (m *Module) emitCall(c *hir.Call) {
	// Built-in print via puts (strings) or print_int (integers)
	if c.Fn == "print" && len(c.Args) == 1 {
		// Special case for string literal
		if s, ok := c.Args[0].(hir.ConstStr); ok {
			// Use printf("%s\n", ...) instead of puts() to handle strings with explicit newlines
			// This way print("hello\n") outputs "hello\n\n" (explicit newline + print's newline)
			m.ensureDecl("declare i32 @printf(ptr, ...)")
			fmtG, fmtN := m.ensureCStringGlobal("%s\n", false)
			wprintf(&m.funcs, "  %%t%d = getelementptr inbounds [%d x i8], [%d x i8]* %s, i64 0, i64 0\n",
				m.tempID, fmtN, fmtN, fmtG)
			strG, strN := m.ensureCStringGlobal(s.Text, false)
			wprintf(&m.funcs, "  %%t%d = getelementptr inbounds [%d x i8], [%d x i8]* %s, i64 0, i64 0\n",
				m.tempID+1, strN, strN, strG)
			wprintf(&m.funcs, "  %%t%d = call i32 (ptr, ...) @printf(ptr %%t%d, ptr %%t%d)\n",
				m.tempID+2, m.tempID, m.tempID+1)
			m.tempID += 3
			return
		}
		// Integer arguments: call print_int
		ty, val := m.operand(c.Args[0])
		if ty == "i32" || ty == "i64" {
			m.ensureDecl("declare void @print_int(i64)")
			// Cast to i64 if needed
			if ty == "i32" {
				wprintf(&m.funcs, "  %%t%d = sext i32 %s to i64\n", m.tempID, val)
				wprintf(&m.funcs, "  call void @print_int(i64 %%t%d)\n", m.tempID)
				m.tempID++
			} else {
				wprintf(&m.funcs, "  call void @print_int(i64 %s)\n", val)
			}
			return
		}
		// Float arguments: call printf with %f
		if ty == "double" {
			m.ensureDecl("declare i32 @printf(ptr, ...)")
			// Create format string global
			g, n := m.ensureCStringGlobal("%f\n", true)
			wprintf(&m.funcs, "  %%t%d = getelementptr inbounds [%d x i8], [%d x i8]* %s, i64 0, i64 0\n", m.tempID, n, n, g)
			fmtPtr := fmt.Sprintf("%%t%d", m.tempID)
			m.tempID++
			wprintf(&m.funcs, "  %%t%d = call i32 (ptr, ...) @printf(ptr %s, double %s)\n", m.tempID, fmtPtr, val)
			m.tempID++
			return
		}
		// Boolean arguments: print true/false
		if ty == "i1" {
			m.ensureDecl("declare i32 @printf(ptr, ...)")
			// Select string based on value
			// Inline implementation with select:
			// %str = select i1 %val, ptr @true_str, ptr @false_str
			// call printf("%s\n", %str)

			trueG, trueN := m.ensureCStringGlobal("true", false)
			falseG, falseN := m.ensureCStringGlobal("false", false)

			wprintf(&m.funcs, "  %%t%d = getelementptr inbounds [%d x i8], [%d x i8]* %s, i64 0, i64 0\n", m.tempID, trueN, trueN, trueG)
			truePtr := fmt.Sprintf("%%t%d", m.tempID)
			m.tempID++

			wprintf(&m.funcs, "  %%t%d = getelementptr inbounds [%d x i8], [%d x i8]* %s, i64 0, i64 0\n", m.tempID, falseN, falseN, falseG)
			falsePtr := fmt.Sprintf("%%t%d", m.tempID)
			m.tempID++

			wprintf(&m.funcs, "  %%t%d = select i1 %s, ptr %s, ptr %s\n", m.tempID, val, truePtr, falsePtr)
			selPtr := fmt.Sprintf("%%t%d", m.tempID)
			m.tempID++

			// Use printf("%s\n", ...) to print the selected string
			fmtG, fmtN := m.ensureCStringGlobal("%s\n", false)
			wprintf(&m.funcs, "  %%t%d = getelementptr inbounds [%d x i8], [%d x i8]* %s, i64 0, i64 0\n", m.tempID, fmtN, fmtN, fmtG)
			fmtPtr := fmt.Sprintf("%%t%d", m.tempID)
			m.tempID++

			wprintf(&m.funcs, "  %%t%d = call i32 (ptr, ...) @printf(ptr %s, ptr %s)\n", m.tempID, fmtPtr, selPtr)
			m.tempID++
			return
		}

		// General case: if arg is ptr, assume it's a string and call printf
		if ty == "ptr" {
			m.ensureDecl("declare i32 @printf(ptr, ...)")
			// Use printf("%s\n", str) to preserve explicit newlines in the string
			fmtG, fmtN := m.ensureCStringGlobal("%s\n", false)
			wprintf(&m.funcs, "  %%t%d = getelementptr inbounds [%d x i8], [%d x i8]* %s, i64 0, i64 0\n",
				m.tempID, fmtN, fmtN, fmtG)
			wprintf(&m.funcs, "  %%t%d = call i32 (ptr, ...) @printf(ptr %%t%d, ptr %s)\n",
				m.tempID+1, m.tempID, val)
			m.tempID += 2
			return
		}
	}

	// Built-in str() conversion
	if c.Fn == "str" && len(c.Args) == 1 {
		ty, val := m.operand(c.Args[0])

		// Determine destination name
		dst := ""
		if c.Dst.Name != "" {
			dst = c.Dst.String()
		} else {
			dst = fmt.Sprintf("%%t%d", m.tempID)
			m.tempID++
		}

		// Helper to register type
		registerType := func(name string) {
			if strings.HasPrefix(name, "%") {
				name = name[1:]
			}
			m.tempTypes[name] = "ptr"
			m.varTypes[name] = types.Str
		}

		// Integer to string
		if ty == "i32" || ty == "i64" {
			m.ensureDecl("declare ptr @int_to_str(i32)")
			if ty == "i64" {
				// Truncate to i32 for now
				truncDst := fmt.Sprintf("%%t%d", m.tempID)
				m.tempID++
				wprintf(&m.funcs, "  %s = trunc i64 %s to i32\n", truncDst, val)
				val = truncDst
			}
			wprintf(&m.funcs, "  %s = call ptr @int_to_str(i32 %s)\n", dst, val)
			registerType(dst)
			return
		}

		// Float to string
		if ty == "double" || ty == "float" {
			m.ensureDecl("declare ptr @float_to_str(double)")
			if ty == "float" {
				extDst := fmt.Sprintf("%%t%d", m.tempID)
				m.tempID++
				wprintf(&m.funcs, "  %s = fpext float %s to double\n", extDst, val)
				val = extDst
			}
			wprintf(&m.funcs, "  %s = call ptr @float_to_str(double %s)\n", dst, val)
			registerType(dst)
			return
		}

		// Bool to string
		if ty == "i1" {
			m.ensureDecl("declare ptr @bool_to_str(i1)")
			wprintf(&m.funcs, "  %s = call ptr @bool_to_str(i1 %s)\n", dst, val)
			registerType(dst)
			return
		}

		// String to string (identity)
		if ty == "ptr" {
			// Check if it's a string or struct
			getType := func(val hir.Value) types.T {
				if v, ok := val.(hir.Var); ok {
					return m.varTypes[v.Name]
				}
				if t, ok := val.(hir.Temp); ok {
					return m.varTypes[t.Name]
				}
				return nil
			}

			argType := getType(c.Args[0])
			isStr := false
			if argType != nil && types.Equal(argType, types.Str) {
				isStr = true
			} else if _, ok := c.Args[0].(hir.ConstStr); ok {
				isStr = true
			}

			if isStr {
				// Identity
				wprintf(&m.funcs, "  %s = bitcast ptr %s to ptr\n", dst, val)
				registerType(dst)
				return
			}

			// Struct to string
			typeName := "Unknown"
			if argType != nil {
				if s, ok := argType.(*types.Struct); ok {
					typeName = s.Name
				} else if cls, ok := argType.(*types.Class); ok {
					typeName = cls.Name
				}
			}

			wprintf(&m.funcs, "  %s = call ptr @%s_to_str(ptr %s)\n", dst, typeName, val)
			m.ensureDecl(fmt.Sprintf("declare ptr @%s_to_str(ptr)", typeName))
			registerType(dst)
			return
		}
	}

	// __future_register_poll(fut, &name$poll, frame)
	if c.Fn == "__future_register_poll" {
		m.ensureDecl("declare void @__future_register_poll(ptr, ptr, ptr)")
		futOp := "ptr null"
		if len(c.Args) > 0 {
			futOp = m.ptrOperand(c.Args[0])
		}
		fnOp := "ptr null"
		if len(c.Args) > 1 {
			if v, ok := c.Args[1].(hir.Var); ok && strings.HasPrefix(v.Name, "&") {
				fnOp = "ptr @" + v.Name[1:]
				if strings.HasSuffix(v.Name, "$poll") {
					base := strings.TrimSuffix(v.Name[1:], "$poll")
					m.MarkAsyncWrapper(base)
				}
			} else {
				fnOp = m.ptrOperand(c.Args[1])
			}
		}
		frameOp := "ptr null"
		if len(c.Args) > 2 {
			frameOp = m.ptrOperand(c.Args[2])
		}
		wprintf(&m.funcs, "  call void @__future_register_poll(%s, %s, %s)\n", futOp, fnOp, frameOp)
		return
	}

	// Fallback: external call — choose return type via overrides/async, honor c.Dst if provided.
	ret := c.Type
	if ret == "" {
		ret = m.callRetType(c.Fn)
	}

	// Build args
	var argStr strings.Builder
	argStr.WriteString("(")
	for i, a := range c.Args {
		if i > 0 {
			argStr.WriteString(", ")
		}
		ty, val := m.operand(a)

		// Use ABI layer to determine if i32 promotion is needed
		abiInfo := abi.Current()
		if abiInfo.NeedsI32ToI64Promotion(c.Fn) && ty == "i32" {
			wprintf(&m.funcs, "  %%t%d = sext i32 %s to i64\n", m.tempID, val)
			val = fmt.Sprintf("%%t%d", m.tempID)
			m.tempID++
			ty = "i64"
		}

		argStr.WriteString(fmt.Sprintf("%s %s", ty, val))
	}
	argStr.WriteString(")")

	// Ensure we have a declare stub. Use ABI layer for variadic signatures.
	// But skip if this function is defined in the same module
	if !m.definedFunctions[c.Fn] {
		abiInfo := abi.Current()
		sig := abiInfo.VariadicSignature(c.Fn, ret)
		if sig != "" {
			m.ensureDecl(sig)
		} else {
			// Generic variadic declaration
			m.ensureDecl(fmt.Sprintf("declare %s @%s(...)", ret, c.Fn))
		}
	}

	if c.Dst.Name != "" {
		dst := c.Dst.String()
		// Use ABI layer to get explicit call syntax if needed
		abiInfo := abi.Current()
		explicitSig := abiInfo.ExplicitCallSyntax(c.Fn, ret)
		if explicitSig != "" {
			wprintf(&m.funcs, "  %s = call %s %s @%s%s\n", dst, ret, explicitSig, c.Fn, argStr.String())
		} else {
			wprintf(&m.funcs, "  %s = call %s @%s%s\n", dst, ret, c.Fn, argStr.String())
		}

		if strings.HasPrefix(dst, "%") {
			m.tempTypes[dst] = ret
			// Register high-level return type if available
			if m.info != nil {
				if set, ok := m.info.Funcs[c.Fn]; ok && len(set.Cands) > 0 {
					dstName := dst
					if strings.HasPrefix(dstName, "%") {
						dstName = dstName[1:]
					}
					m.varTypes[dstName] = set.Cands[0].Type.Ret
				}
			}
		}
		if strings.HasPrefix(dst, "%t") {
			if n, err := strconv.Atoi(dst[2:]); err == nil {
				if n >= m.tempID {
					m.tempID = n + 1
				}
			}
		}
		return
	}

	if ret == "void" {
		// Use ABI layer to get explicit call syntax if needed
		abiInfo := abi.Current()
		explicitSig := abiInfo.ExplicitCallSyntax(c.Fn, ret)
		if explicitSig != "" {
			wprintf(&m.funcs, "  call %s %s @%s%s\n", ret, explicitSig, c.Fn, argStr.String())
		} else {
			wprintf(&m.funcs, "  call %s @%s%s\n", ret, c.Fn, argStr.String())
		}
		return
	}

	// Use ABI layer to get explicit call syntax if needed
	abiInfo := abi.Current()
	explicitSig := abiInfo.ExplicitCallSyntax(c.Fn, ret)
	if explicitSig != "" {
		wprintf(&m.funcs, "  %%t%d = call %s %s @%s%s\n", m.tempID, ret, explicitSig, c.Fn, argStr.String())
	} else {
		wprintf(&m.funcs, "  %%t%d = call %s @%s%s\n", m.tempID, ret, c.Fn, argStr.String())
	}
	tempName := fmt.Sprintf("t%d", m.tempID)
	m.tempTypes[tempName] = ret

	// Register high-level return type if available
	if m.info != nil {
		if set, ok := m.info.Funcs[c.Fn]; ok && len(set.Cands) > 0 {
			// Use the return type of the first candidate (sufficient for builtins like str)
			// TODO: Handle overloads with different return types if that ever happens
			m.varTypes[tempName] = set.Cands[0].Type.Ret
		}
	}

	m.tempID++
}

// operand infers the (type, value) pair for a generic value.
func (m *Module) operand(v hir.Value) (string, string) {
	switch t := v.(type) {
	case hir.ConstInt:
		ty := t.Type
		if ty == "" {
			ty = "i32"
		}
		return ty, t.Text
	case hir.ConstFloat:
		return "double", t.Text
	case hir.ConstBool:
		return "i1", fmt.Sprintf("%v", t.Value)
	case hir.ConstStr:
		// Strings are pointers to globals
		// Don't add newline here - puts() will add it if needed
		g, n := m.ensureCStringGlobal(t.Text, false)
		// We need to emit a GEP to get the pointer
		// But operand() is called inside a printf, we can't emit instructions here easily!
		// Wait, emitCall builds the string.
		// We can't emit instructions inside the argument list construction if we are just returning strings.
		// We need to pre-emit the GEP if it's a string literal.
		// Hack: return the global array directly? No, need i8*.
		// We can use `i8* getelementptr ...` constant expression!
		return "ptr", fmt.Sprintf("getelementptr inbounds ([%d x i8], [%d x i8]* %s, i64 0, i64 0)", n, n, g)
	case hir.Temp:
		// Infer type from source
		name := t.Name
		if !strings.HasPrefix(name, "%") {
			name = "%" + name
		}
		return m.inferType(t.Name), name
	case hir.Var:
		if ali, ok := m.ssa[t.Name]; ok {
			return m.operand(ali)
		}
		// Check if we have type info for this variable (including function parameters)
		ty := m.inferType(t.Name)
		return ty, "%" + t.Name
	case hir.Undef:
		return "undef", "undef"
	default:
		return "i32", "0"
	}
}

func (m *Module) inferType(name string) string {
	// Check tempTypes with the full name (including %)
	if ty, ok := m.tempTypes[name]; ok {
		return ty
	}

	key := strings.TrimPrefix(name, "%")

	// Check tempTypes with stripped name
	if ty, ok := m.tempTypes[key]; ok {
		return ty
	}

	// Check ssa with stripped name
	if v, ok := m.ssa[key]; ok {
		ty, _ := m.operand(v)
		return ty
	}

	// Check if it was a Call result
	// We don't track which temp came from which call easily.
	// But we can assume i32 default.
	return "i32"
}

func (m *Module) emitRet(r *hir.Ret) {
	if r == nil {
		return
	}
	// If there's no explicit value, return a typed zero consistent with the current function header.
	if r.Val == nil {
		switch m.curFuncRetTy {
		case "ptr":
			wprintf(&m.funcs, "  ret ptr null\n")
		case "float":
			wprintf(&m.funcs, "  ret float 0.0\n")
		case "double":
			wprintf(&m.funcs, "  ret double 0.0\n")
		case "i1":
			wprintf(&m.funcs, "  ret i1 0\n")
		case "":
			// No override registered → Tier-0 default.
			wprintf(&m.funcs, "  ret i32 0\n")
		default:
			// Any integer width: i8/i16/i32/i64/i128, usize/isize lowered earlier.
			wprintf(&m.funcs, "  ret %s 0\n", m.curFuncRetTy)
		}
		return
	}

	// Non-nil return: use the function's return type
	if m.curRetIsPtr || m.curFuncRetTy == "ptr" {
		wprintf(&m.funcs, "  ret %s\n", m.ptrOperand(r.Val))
		return
	}

	// Use the actual return type from the function signature
	retTy := m.curFuncRetTy
	if retTy == "" {
		retTy = "i32" // default
	}
	ty, val := m.operand(r.Val)
	_ = ty // We use retTy from function signature, not operand type
	wprintf(&m.funcs, "  ret %s %s\n", retTy, val)
}

// ptrOperand renders a pointer-typed operand, honoring SSA aliases for vars.
func (m *Module) ptrOperand(v hir.Value) string {
	switch t := v.(type) {
	case hir.Temp:
		return fmt.Sprintf("ptr %s", t.Name)
	case hir.ConstStr:
		g, n := m.ensureCStringGlobal(t.Text, true)
		return fmt.Sprintf("ptr getelementptr inbounds ([%d x i8], [%d x i8]* %s, i64 0, i64 0)", n, n, g)
	case hir.Var:
		if ali, ok := m.ssa[t.Name]; ok {
			return m.ptrOperand(ali)
		}
		return fmt.Sprintf("ptr %%%s", t.Name)
	default:
		return "ptr null"
	}
}

// i32Operand renders an i32 operand, honoring SSA aliases for vars.
func (m *Module) i32Operand(v hir.Value) string {
	switch t := v.(type) {
	case hir.ConstInt:
		return fmt.Sprintf("i32 %s", t.Text)
	case hir.Temp:
		return fmt.Sprintf("i32 %s", t.Name)
	case hir.Var:
		if ali, ok := m.ssa[t.Name]; ok {
			return m.i32Operand(ali)
		}
		return fmt.Sprintf("i32 %%%s", t.Name)
	default:
		return "i32 0"
	}
}

// callRetType returns the textual LLVM return type for calls to 'name':
//   - async wrappers => ptr
//   - override via getFuncSig (if present)
//   - default i32 (Tier-0)
func (m *Module) callRetType(name string) string {
	if m.asyncWrappers[name] {
		return "ptr"
	}
	if sig, ok := getFuncSig(name); ok && sig.ret != "" {
		return sig.ret
	}
	return "i32"
}

// MarkDefined marks a function as defined in this module.
func (m *Module) MarkDefined(name string) {
	m.definedFunctions[name] = true
}
func (m *Module) emitInsertValue(x *hir.InsertValue) {
	aggTy, aggVal := m.operand(x.Agg)
	if x.Type != nil {
		if s, ok := x.Type.(string); ok && s != "" {
			aggTy = s
		} else if t, ok := x.Type.(types.T); ok {
			aggTy = LowerPrimType(t)
		}
	}

	elemTy, elemVal := m.operand(x.Elem)

	// Use fmt.Fprintf directly to avoid percent escaping in aggVal (e.g., %tup1)
	// x.Dst.Name already includes %% from FreshTemp
	fmt.Fprintf(&m.funcs, "  %s = insertvalue %s %s, %s %s, %d\n",
		x.Dst.Name, aggTy, aggVal, elemTy, elemVal, x.Index)

	m.tempTypes[x.Dst.Name] = aggTy
}

func (m *Module) emitExtractValue(x *hir.ExtractValue) {
	aggTy, aggVal := m.operand(x.Agg)
	// We don't need x.Type for the instruction, but we need it for m.tempTypes

	var resTy string
	if x.Type != nil {
		if s, ok := x.Type.(string); ok && s != "" {
			resTy = s
		} else if t, ok := x.Type.(types.T); ok {
			resTy = LowerPrimType(t)
		}
	}

	// Use fmt.Fprintf directly to avoid percent escaping
	// x.Dst.Name already includes %% from FreshTemp
	fmt.Fprintf(&m.funcs, "  %s = extractvalue %s %s, %d\n",
		x.Dst.Name, aggTy, aggVal, x.Index)

	m.tempTypes[x.Dst.Name] = resTy
}

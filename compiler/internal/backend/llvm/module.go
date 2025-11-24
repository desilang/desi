package llvm

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/desilang/desi/compiler/internal/backend/llvm/abi"
	"github.com/desilang/desi/compiler/internal/backend/llvm/intrin"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/term"
)

// wprintf writes formatted text while ignoring write errors.
func wprintf(w io.Writer, format string, a ...any) {
	term.Write(w, []byte(fmt.Sprintf(format, a...)))
}

type Module struct {
	name             string
	globals          bytes.Buffer
	funcs            bytes.Buffer
	strLits          map[string]string // text-key -> global name
	strOrder         []string          // deterministic order
	needPuts         bool
	needRcDec        bool
	needArena        bool
	wroteGlob        bool
	tempID           int
	asyncWrappers    map[string]bool      // symbols that return ptr future handles
	curRetIsPtr      bool                 // set per function during EmitFunc
	ssa              map[string]hir.Value // simple name -> value alias (lets/assigns + frame slots)
	curFuncRetTy     string               // textual LLVM return type for the function being emitted
	tempTypes        map[string]string    // temp name -> llvm type (e.g. "t1" -> "ptr")
	definedFunctions map[string]bool      // functions defined in this module (to avoid duplicate declares)
}

func NewModule(name string) *Module {
	return &Module{
		name:             name,
		strLits:          make(map[string]string),
		asyncWrappers:    make(map[string]bool),
		definedFunctions: make(map[string]bool),
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
func (m *Module) EmitFunc(fn *hir.Func) {
	// Mark this function as defined to avoid emitting a declare for it
	m.definedFunctions[fn.Name] = true

	// Reset per-function state.
	m.ssa = make(map[string]hir.Value)
	m.tempTypes = make(map[string]string)
	m.curRetIsPtr = m.asyncWrappers[fn.Name]

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
	}
	wprintf(&m.funcs, ") {\n")

	type localInfo struct {
		name string
		size int
	}
	for bi, b := range fn.Blocks {
		label := b.Name
		if bi == 0 && (label == "" || label == "entry") {
			label = "entry"
		}
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
				// Keep allocas + lifetimes for all locals to satisfy existing tests.
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
				// Tier-0 no-op
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
				valTy, valOp := m.operand(x.Val)
				ptrOp := m.ptrOperand(x.Dst)
				wprintf(&m.funcs, "  store %s %s, %s\n", valTy, valOp, ptrOp)

			case *hir.Load:
				ptrOp := m.ptrOperand(x.Src)
				wprintf(&m.funcs, "  %s = load %s, %s\n", x.Dst.Name, x.Type, ptrOp)
				m.tempTypes[x.Dst.Name] = x.Type

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

			// ------- control flow (elided) -------
			case *hir.If:
			case *hir.While:

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
	ret := m.callRetType(c.Fn)

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
	m.tempTypes[fmt.Sprintf("t%d", m.tempID)] = ret
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
		return m.inferType(t.Name), t.Name
	case hir.Var:
		if ali, ok := m.ssa[t.Name]; ok {
			return m.operand(ali)
		}
		// Var is usually a pointer (alloca).
		// But if we want the value, we should have loaded it?
		// Tier-0 uses alloca for everything.
		// If we pass a Var, we usually pass the pointer (by ref) or load it?
		// Desi passes by value for primitives, by ref for others?
		// For M14, let's assume we pass the value.
		// But we haven't emitted a load!
		// The Var `p` is `alloca i32`.
		// We need to load it to pass it?
		// Or pass the pointer?
		// `Point_to_str(p)` expects `ptr` (self).
		// If `p` is `alloca`, then `%p` is `ptr`.
		// So passing `%p` is correct for `self`.
		return "ptr", "%" + t.Name
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

	// Non-nil return: keep Tier-0 behavior; if current func is a ptr-returner (async wrapper),
	// format as a pointer; otherwise use the i32-operand path to keep existing tests stable.
	if m.curRetIsPtr || m.curFuncRetTy == "ptr" {
		wprintf(&m.funcs, "  ret %s\n", m.ptrOperand(r.Val))
		return
	}
	wprintf(&m.funcs, "  ret %s\n", m.i32Operand(r.Val))
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

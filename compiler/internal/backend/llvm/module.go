package llvm

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/desilang/desi/compiler/internal/backend/llvm/intrin"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/term"
)

// wprintf writes formatted text while ignoring write errors.
func wprintf(w io.Writer, format string, a ...any) {
	term.Write(w, []byte(fmt.Sprintf(format, a...)))
}

type Module struct {
	name          string
	globals       bytes.Buffer
	funcs         bytes.Buffer
	strLits       map[string]string // text-key -> global name
	strOrder      []string          // deterministic order
	needPuts      bool
	needRcDec     bool
	needArena     bool
	wroteGlob     bool
	tempID        int
	asyncWrappers map[string]bool      // symbols that return ptr future handles
	curRetIsPtr   bool                 // set per function during EmitFunc
	ssa           map[string]hir.Value // simple name -> value alias (lets/assigns + frame slots)
}

func NewModule(name string) *Module {
	return &Module{
		name:          name,
		strLits:       make(map[string]string),
		asyncWrappers: make(map[string]bool),
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
		m.ensureDecl("declare i32 @puts(i8*)")
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

// EmitFunc: Tier-0 subset—calls, returns, lifetimes for locals.
// Task K refinement:
//   - Emit lifetime.end for block locals immediately *before* an unconditional 'ret'.
//   - Do NOT emit lifetime.end *after* the 'ret'.
func (m *Module) EmitFunc(fn *hir.Func) {
	// Reset per-function state.
	m.ssa = make(map[string]hir.Value)
	m.curRetIsPtr = m.asyncWrappers[fn.Name]

	retTy := "i32"
	if m.curRetIsPtr {
		retTy = "ptr"
	}

	// Params (Tier-0): lower all params as ptr.
	if len(fn.Params) == 0 {
		wprintf(&m.funcs, "define %s @%s() {\n", retTy, fn.Name)
	} else {
		wprintf(&m.funcs, "define %s @%s(", retTy, fn.Name)
		for i, p := range fn.Params {
			if i > 0 {
				wprintf(&m.funcs, ", ")
			}
			wprintf(&m.funcs, "ptr %%%s", p.Name)
		}
		wprintf(&m.funcs, ") {\n")
	}

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

		// If we didn't already close lifetimes (i.e., no ret), close them now.
		if !lifetimesClosed {
			for i := len(locals) - 1; i >= 0; i-- {
				li := locals[i]
				wprintf(&m.funcs, "%s", intrin.LifetimeEnd(li.size, li.name))
			}
		}
	}
	wprintf(&m.funcs, "}\n")
}

func (m *Module) emitCall(c *hir.Call) {
	// Built-in print via puts
	if c.Fn == "print" && len(c.Args) == 1 {
		if s, ok := c.Args[0].(hir.ConstStr); ok {
			g, n := m.ensureCStringGlobal(s.Text, true)
			wprintf(&m.funcs, "  %%t%d = getelementptr inbounds [%d x i8], [%d x i8]* %s, i64 0, i64 0\n",
				m.tempID, n, n, g)
			wprintf(&m.funcs, "  %%t%d = call i32 @puts(i8* %%t%d)\n", m.tempID+1, m.tempID)
			m.tempID += 2
			m.needPuts = true
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

	// Fallback: external call — choose return type, honor c.Dst if provided.
	ret := "i32"
	if m.asyncWrappers[c.Fn] {
		ret = "ptr"
	}

	if c.Dst.Name != "" {
		dst := c.Dst.String()
		wprintf(&m.funcs, "  %s = call %s @%s()\n", dst, ret, c.Fn)
		if strings.HasPrefix(dst, "%t") {
			if n, err := strconv.Atoi(dst[2:]); err == nil {
				if n >= m.tempID {
					m.tempID = n + 1
				}
			}
		}
		return
	}

	wprintf(&m.funcs, "  %%t%d = call %s @%s()\n", m.tempID, ret, c.Fn)
	m.tempID++
}

func (m *Module) emitRet(r *hir.Ret) {
	// Pointer-returning wrapper?
	if m.curRetIsPtr {
		if r.Val == nil {
			wprintf(&m.funcs, "  ret ptr null\n")
			return
		}
		switch v := r.Val.(type) {
		case hir.Temp:
			wprintf(&m.funcs, "  ret ptr %s\n", v.String())
		case hir.Var:
			if ali, ok := m.ssa[v.Name]; ok {
				if a, ok2 := ali.(hir.Temp); ok2 {
					wprintf(&m.funcs, "  ret ptr %s\n", a.String())
					return
				}
			}
			wprintf(&m.funcs, "  ret ptr %%%s\n", v.Name)
		default:
			wprintf(&m.funcs, "  ret ptr null\n")
		}
		return
	}

	// Default i32 path.
	if r.Val == nil {
		wprintf(&m.funcs, "  ret i32 0\n")
		return
	}
	switch v := r.Val.(type) {
	case hir.ConstInt:
		wprintf(&m.funcs, "  ret i32 %s\n", v.Text)
	case hir.Temp:
		wprintf(&m.funcs, "  ret i32 %s\n", v.String())
	case hir.Var:
		wprintf(&m.funcs, "  ret %s\n", m.i32Operand(v))
	default:
		wprintf(&m.funcs, "  ret i32 0\n")
	}
}

// ptrOperand renders a pointer-typed operand, honoring SSA aliases for vars.
func (m *Module) ptrOperand(v hir.Value) string {
	switch t := v.(type) {
	case hir.Temp:
		return fmt.Sprintf("ptr %s", t.Name)
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

package llvm

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/desilang/desi/compiler/internal/backend/llvm/abi"
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
	curFunc            *hir.Func
	tempID             int             // counter for %t0, %t1, ...
	mergeID            int             // counter for merge blocks
	asyncWrappers      map[string]bool // functions returning ptr (future handle)
	definedFunctions   map[string]bool // track which functions we've defined
	needPuts           bool
	needRcDec          bool
	needArena          bool
	curFuncRetTy       string
	curRetIsPtr        bool
	cfBlocks           map[string]string    // blocks that need terminators to merge labels
	cfLoopConds        map[string]hir.Value // loop condition blocks -> condition value
	wroteGlob          bool
	info               *check.Info       // type checker info for move analysis
	currentMoves       map[string]bool   // moved variables in current function
	staticFieldGlobals map[string]string // name -> LLVM type (e.g., "@Counter_count" -> "i32")
}

func NewModule(name string) *Module {
	return &Module{
		name:               name,
		strLits:            make(map[string]string),
		asyncWrappers:      make(map[string]bool),
		definedFunctions:   make(map[string]bool),
		cfBlocks:           make(map[string]string),
		cfLoopConds:        make(map[string]hir.Value),
		staticFieldGlobals: make(map[string]string),
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

	// Emit static field globals
	for name, llvmType := range m.staticFieldGlobals {
		wprintf(&m.globals, "%s = global %s 0, align 4\n", name, llvmType)
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
		// Special case: "null" should be emitted as null pointer, not string literal
		if t.Text == "null" {
			return "ptr null"
		}
		// Otherwise, normal string constant
		g, n := m.ensureCStringGlobal(t.Text, true)
		return fmt.Sprintf("ptr getelementptr inbounds ([%d x i8], [%d x i8]* %s, i64 0, i64 0)", n, n, g)
	case hir.Var:
		if ali, ok := m.ssa[t.Name]; ok {
			return m.ptrOperand(ali)
		}
		// Check if it's a global variable (starts with @)
		if strings.HasPrefix(t.Name, "@") {
			return fmt.Sprintf("ptr %s", t.Name)
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

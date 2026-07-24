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
	curFuncName        string          // name of current function for depth tracking
	tempID             int             // counter for %t0, %t1, ...
	mergeID            int             // counter for merge blocks
	asyncWrappers      map[string]bool // functions returning ptr (future handle)
	definedFunctions   map[string]bool // functions that will be defined (prevents extern declares)
	emittedFunctions   map[string]bool // functions that have been emitted (prevents duplicates)
	needPuts           bool
	needRcDec          bool
	needArena          bool
	curFuncRetTy       string
	curRetIsPtr        bool
	cfBlocks           map[string]string    // blocks that need terminators to merge labels
	cfLoopConds        map[string]hir.Value // loop condition blocks -> condition value
	wroteGlob          bool
	info               *check.Info          // type checker info for move analysis
	currentMoves       map[string]bool      // moved variables in current function
	staticFieldGlobals map[string]GlobalDef // name -> definition
	nameVersions       map[string]int       // track name usage for unique SSA names

	// Lazy module initialization tracking
	lazyModules     map[string]bool // module path -> needs lazy init
	lazyInitEmitted map[string]bool // module path -> init thunk already emitted

	// TaskGroup wrapper functions: wrapperName -> WrapperInfo
	tgWrappers map[string]WrapperInfo
}

// WrapperInfo tracks info needed to emit a TaskGroup wrapper function
type WrapperInfo struct {
	TargetFn    string // function to call (__lam$N or named function)
	NumCaptures int    // number of captures to unpack from ctx
}

type GlobalDef struct {
	Type  string
	Value string
}

func NewModule(name string) *Module {
	return &Module{
		name:               name,
		strLits:            make(map[string]string),
		asyncWrappers:      make(map[string]bool),
		definedFunctions:   make(map[string]bool),
		emittedFunctions:   make(map[string]bool),
		ssa:                make(map[string]hir.Value),
		tempTypes:          make(map[string]string),
		varTypes:           make(map[string]types.T),
		cfBlocks:           make(map[string]string),
		cfLoopConds:        make(map[string]hir.Value),
		currentMoves:       make(map[string]bool),
		staticFieldGlobals: make(map[string]GlobalDef),
		nameVersions:       make(map[string]int),
		lazyModules:        make(map[string]bool),
		lazyInitEmitted:    make(map[string]bool),
		tgWrappers:         make(map[string]WrapperInfo),
	}
}

// MarkAsyncWrapper records that calls to 'name' return a ptr (future handle).
func (m *Module) MarkAsyncWrapper(name string) {
	m.asyncWrappers[name] = true
}

// SetLazyModules registers modules for lazy initialization.
// Each module will get an init flag and thunk generated.
func (m *Module) SetLazyModules(modules []string) {
	for _, mod := range modules {
		m.lazyModules[mod] = true
	}
}

// sanitizeModName converts a module path to a valid LLVM identifier.
// Replaces dots and slashes with underscores.
func sanitizeModName(path string) string {
	s := strings.ReplaceAll(path, ".", "_")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "-", "_")
	return s
}

// emitLazyInitThunks generates lazy init flags and thunks for registered modules.
// Called before IR() to emit the necessary globals and functions.
func (m *Module) emitLazyInitThunks() {
	for modPath := range m.lazyModules {
		if m.lazyInitEmitted[modPath] {
			continue
		}
		m.lazyInitEmitted[modPath] = true

		safeName := sanitizeModName(modPath)
		flagName := fmt.Sprintf("@__mod_%s_init_done", safeName)
		thunkName := fmt.Sprintf("__ensure_%s_init", safeName)
		topName := fmt.Sprintf("%s___top__", safeName) // Module's __top__ function

		// Emit global init flag
		wprintf(&m.globals, "%s = internal global i1 false\n", flagName)

		// Emit init thunk function
		var b bytes.Buffer
		wprintf(&b, "define void @%s() {\n", thunkName)
		wprintf(&b, "entry:\n")
		wprintf(&b, "  %%done = load i1, ptr %s\n", flagName)
		wprintf(&b, "  br i1 %%done, label %%skip, label %%init\n")
		wprintf(&b, "\n")
		wprintf(&b, "init:\n")
		// Call the module's __top__ function if it exists
		if m.definedFunctions[topName] {
			wprintf(&b, "  call i32 @%s()\n", topName)
		}
		wprintf(&b, "  store i1 true, ptr %s\n", flagName)
		wprintf(&b, "  br label %%skip\n")
		wprintf(&b, "\n")
		wprintf(&b, "skip:\n")
		wprintf(&b, "  ret void\n")
		wprintf(&b, "}\n\n")
		m.funcs.Write(b.Bytes())
	}
}

// ensureLazyModuleInit checks if a function belongs to a lazy module and emits
// the init thunk call if it hasn't been called yet for this module.
// Returns true if the function belongs to a lazy module (init was ensured),
// false otherwise.
func (m *Module) ensureLazyModuleInit(fnName string) bool {
	// Check each lazy module to see if the function starts with its prefix
	for modPath := range m.lazyModules {
		safeName := sanitizeModName(modPath)
		prefix := safeName + "_"

		// Check if function starts with module prefix (e.g., "math_sin" for "math" module)
		if strings.HasPrefix(fnName, prefix) {
			// Only emit the init call once per module
			thunkName := fmt.Sprintf("__ensure_%s_init", safeName)
			if !m.lazyInitEmitted[modPath+"_called"] {
				m.lazyInitEmitted[modPath+"_called"] = true
				// Don't emit declare - the thunk is defined in emitLazyInitThunks()
				// LLVM doesn't allow declare+define for the same function
				wprintf(&m.funcs, "  call void @%s()\n", thunkName)
			}
			return true
		}
	}
	return false
}

// uniqueName generates a unique SSA name by appending a version suffix if needed.
// This handles variables with the same name appearing in multiple scopes (e.g., loop variables).
// Call once per variable definition, then use the returned name consistently.
func (m *Module) uniqueName(name string) string {
	ver := m.nameVersions[name]
	m.nameVersions[name] = ver + 1
	if ver == 0 {
		return name
	}
	return fmt.Sprintf("%s_%d", name, ver)
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
	if idx := strings.Index(line, "@"); idx != -1 {
		if openParen := strings.Index(line[idx:], "("); openParen != -1 {
			fnPattern := line[idx : idx+openParen+1] // e.g. "@string_concat("
			if strings.Contains(m.globals.String(), fnPattern) {
				return
			}
		}
	}
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
	for name, def := range m.staticFieldGlobals {
		wprintf(&m.globals, "%s = global %s %s, align 4\n", name, def.Type, def.Value)
	}
}

func (m *Module) IR() string {
	m.writeGlobals()
	m.emitTGWrappers()     // Emit TaskGroup wrapper functions
	m.emitLazyInitThunks() // Emit lazy module init thunks

	// Windows script-mode fix: if this is a Windows target and no @main was
	// defined (script-mode file with top-level statements but no def main()),
	// emit a @main wrapper that calls __desi_runtime_init() then @__top__().
	//
	// On POSIX, entry.c in libdesi.lib provides this exact wrapper and the
	// linker pulls it in automatically. On Windows, the MSVC linker only pulls
	// lib members that resolve *undefined* symbols. Since @__top__ is DEFINED
	// in the IR (not undefined), entry.obj is never pulled in → LNK1561.
	//
	// Emitting @main directly in the IR is the cleanest solution and matches
	// what entry.c does without requiring any linker flag changes.
	abiInfo := abi.Current()
	if abiInfo.IsWindows() && !m.definedFunctions["main"] && m.definedFunctions["__top__"] {
		m.ensureDecl("declare void @__desi_runtime_init()")
		var b bytes.Buffer
		wprintf(&b, "define i32 @main() {\n")
		wprintf(&b, "entry:\n")
		wprintf(&b, "  call void @__desi_runtime_init()\n")
		wprintf(&b, "  call i32 @__top__()\n")
		wprintf(&b, "  ret i32 0\n")
		wprintf(&b, "}\n\n")
		m.funcs.Write(b.Bytes())
	}

	var out bytes.Buffer
	out.WriteString(fmt.Sprintf("target datalayout = \"%s\"\n", abiInfo.TargetLayout))
	out.WriteString(fmt.Sprintf("target triple = \"%s\"\n\n", abiInfo.TargetTriple))

	out.Write(m.globals.Bytes())
	out.WriteString("\n")
	out.Write(m.funcs.Bytes())
	return out.String()
}

// RegisterTGWrapper registers a TaskGroup wrapper function to be emitted
func (m *Module) RegisterTGWrapper(wrapperName, targetFn string, numCaptures int) {
	if m.tgWrappers[wrapperName].TargetFn != "" {
		return // Already registered
	}
	m.tgWrappers[wrapperName] = WrapperInfo{
		TargetFn:    targetFn,
		NumCaptures: numCaptures,
	}
}

// emitTGWrappers emits all registered TaskGroup wrapper functions as LLVM IR.
// Reads from the module's local registry.
func (m *Module) emitTGWrappers() {
	for wrapperName, info := range m.tgWrappers {
		m.emitSingleWrapper(wrapperName, info.TargetFn, info.NumCaptures)
	}
}

// EmitGlobalTGWrappers emits wrappers from the global registry.
// Called by emit_ir_cmd after all lowering is complete.
func (m *Module) EmitGlobalTGWrappers(wrappers map[string]struct {
	TargetFn    string
	NumCaptures int
}) {
	for wrapperName, info := range wrappers {
		// Skip if already emitted from local registry
		if m.tgWrappers[wrapperName].TargetFn != "" {
			continue
		}
		m.emitSingleWrapper(wrapperName, info.TargetFn, info.NumCaptures)
	}
}

// emitSingleWrapper emits one wrapper function
func (m *Module) emitSingleWrapper(wrapperName, targetFn string, numCaptures int) {
	// Generate wrapper: define void @wrapperName(ptr %__ctx__) { ... }
	var b bytes.Buffer
	wprintf(&b, "define void @%s(ptr %%__ctx__) {\n", wrapperName)
	wprintf(&b, "entry:\n")

	if numCaptures == 0 {
		// No captures: just call target function ignoring ctx
		wprintf(&b, "  call void @%s()\n", targetFn)
	} else {
		// Unpack captures from ctx and call target
		for i := 0; i < numCaptures; i++ {
			// Get pointer to capture[i]: getelementptr i8, ptr %__ctx__, i64 (i*8)
			wprintf(&b, "  %%cap%d_ptr = getelementptr i8, ptr %%__ctx__, i64 %d\n", i, i*8)
			// Load the value
			wprintf(&b, "  %%cap%d = load ptr, ptr %%cap%d_ptr\n", i, i)
		}
		// Call target with captures
		wprintf(&b, "  call void @%s(", targetFn)
		for i := 0; i < numCaptures; i++ {
			if i > 0 {
				wprintf(&b, ", ")
			}
			wprintf(&b, "ptr %%cap%d", i)
		}
		wprintf(&b, ")\n")
	}

	wprintf(&b, "  ret void\n")
	wprintf(&b, "}\n")

	m.funcs.Write(b.Bytes())
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
// RegisterFunc pre-registers a function name so calls to it won't get
// incorrect variadic declarations. Call this for all functions before
// emitting any function bodies.
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
	case hir.ConstNull:
		// Null pointer constant
		return "ptr", "null"
	case hir.FuncRef:
		// Function pointer reference
		return "ptr", "@" + t.Name
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
		// Global functions/variables start with @, don't add % prefix
		if strings.HasPrefix(t.Name, "@") {
			return "ptr", t.Name
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

	// Emit call depth decrement for user functions (skip main/__top__)
	if m.curFuncName != "" && m.curFuncName != "main" && !strings.HasSuffix(m.curFuncName, "__top__") {
		m.ensureDecl("declare void @__desi_call_exit()")
		wprintf(&m.funcs, "  call void @__desi_call_exit()\n")
	}

	// If there's no explicit value, return a typed zero consistent with the current function header.
	if r.Val == nil {
		switch m.curFuncRetTy {
		case "void":
			wprintf(&m.funcs, "  ret void\n")
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

	// Auto-widen if function returns i64 but operand is narrower (async body)
	if retTy == "i64" && ty != "i64" {
		switch ty {
		case "i32":
			tmp := fmt.Sprintf("%%ret_sext_%d", m.tempID)
			m.tempID++
			wprintf(&m.funcs, "  %s = sext i32 %s to i64\n", tmp, val)
			wprintf(&m.funcs, "  ret i64 %s\n", tmp)
			return
		case "ptr":
			tmp := fmt.Sprintf("%%ret_p2i_%d", m.tempID)
			m.tempID++
			wprintf(&m.funcs, "  %s = ptrtoint ptr %s to i64\n", tmp, val)
			wprintf(&m.funcs, "  ret i64 %s\n", tmp)
			return
		case "i1":
			tmp := fmt.Sprintf("%%ret_zext_%d", m.tempID)
			m.tempID++
			wprintf(&m.funcs, "  %s = zext i1 %s to i64\n", tmp, val)
			wprintf(&m.funcs, "  ret i64 %s\n", tmp)
			return
		case "double":
			tmp := fmt.Sprintf("%%ret_d2i_%d", m.tempID)
			m.tempID++
			wprintf(&m.funcs, "  %s = bitcast double %s to i64\n", tmp, val)
			wprintf(&m.funcs, "  ret i64 %s\n", tmp)
			return
		}
	}

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
		g, n := m.ensureCStringGlobal(t.Text, false)
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

// DefineGlobal registers a global variable definition.
// It will be emitted in the module header alongside string literals.
func (m *Module) DefineGlobal(name, typ, val string) {
	if val == "" {
		val = "0"
		if typ == "ptr" {
			val = "null"
		} else if typ == "float" || typ == "double" {
			val = "0.0"
		}
	} else if typ == "ptr" && val != "null" && !strings.HasPrefix(val, "@") && !strings.HasPrefix(val, "getelementptr") {
		// Assume val is the string content
		// We call ensureCStringGlobal which returns the @.str global name
		g, n := m.ensureCStringGlobal(val, false)
		val = fmt.Sprintf("getelementptr inbounds ([%d x i8], [%d x i8]* %s, i64 0, i64 0)", n, n, g)
	}
	m.staticFieldGlobals[name] = GlobalDef{Type: typ, Value: val}
}

// RegisterFunc declares a function that will be defined later in this module.
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

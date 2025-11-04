package llvm

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/term"
)

// wprintf writes formatted text to any io.Writer while ignoring write errors.
func wprintf(w io.Writer, format string, a ...any) {
	term.Write(w, []byte(fmt.Sprintf(format, a...)))
}

type Module struct {
	name      string
	globals   bytes.Buffer
	funcs     bytes.Buffer
	strLits   map[string]string // text-key -> global name
	strOrder  []string          // deterministic order
	needPuts  bool
	wroteGlob bool
	tempID    int
}

func NewModule(name string) *Module {
	return &Module{
		name:    name,
		strLits: make(map[string]string),
	}
}

func (m *Module) nextStrName() string {
	return fmt.Sprintf("@.str.%d", len(m.strOrder))
}

// ensureCStringGlobal(text) creates/reuses a private unnamed_addr constant.
// If withNewline is true and text lacks '\n', we append one; we always add the NUL.
// Returns the global name and element count (array length).
func (m *Module) ensureCStringGlobal(text string, withNewline bool) (gname string, count int) {
	payload := text
	if withNewline && !strings.HasSuffix(payload, "\n") {
		payload += "\n"
	}
	count = len(payload) + 1
	key := fmt.Sprintf("%d:%s", count, payload)
	if name, ok := m.strLits[key]; ok {
		return name, count
	}
	name := m.nextStrName()
	m.strLits[key] = name
	m.strOrder = append(m.strOrder, key)
	return name, count
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
		s := it.key[strings.Index(it.key, ":")+1:] // strip index
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
		wprintf(&m.globals, "declare i32 @puts(i8*)\n")
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

// EmitFunc: Tier-0 subset—calls, returns, conservative lifetimes for locals.
func (m *Module) EmitFunc(fn *hir.Func) {
	wprintf(&m.funcs, "define i32 @%s() {\n", fn.Name)
	for bi, b := range fn.Blocks {
		label := b.Name
		if bi == 0 && (label == "" || label == "entry") {
			label = "entry"
		}
		wprintf(&m.funcs, "%s:\n", label)

		var locals []string
		for _, s := range b.Stmts {
			switch x := s.(type) {
			case *hir.Let:
				// Materialize locals on the stack and wrap lifetime (conservative Tier-0).
				llvmTy := "i32"
				size := 4
				switch x.Init.(type) {
				case hir.ConstBool:
					llvmTy, size = "i1", 1
				case hir.ConstStr:
					llvmTy, size = "ptr", 8
				case hir.ConstInt:
					llvmTy, size = "i32", 4
				}
				wprintf(&m.funcs, "  %%%s = alloca %s\n", x.Name, llvmTy)
				wprintf(&m.funcs, "  call void @llvm.lifetime.start.p0(i64 %d, ptr %%%s)\n", size, x.Name)
				locals = append(locals, x.Name)

			case *hir.Call:
				m.emitCall(x)

			case *hir.Ret:
				m.emitRet(x)

				// Tier-0: Drop/DecRef/DestroyArena are handled at codegen sites; no-op here
			}
		}
		// Close lifetimes at block end (Tier-0: whole-function bracketing).
		for i := len(locals) - 1; i >= 0; i-- {
			name := locals[i]
			wprintf(&m.funcs, "  call void @llvm.lifetime.end.p0(i64 4, ptr %%%s)\n", name)
		}
	}
	wprintf(&m.funcs, "}\n")
}

func (m *Module) emitCall(c *hir.Call) {
	// Built-in print: print("…") → puts("…\n")
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
	// Fallback: call external by name, drop args (Tier-0)
	wprintf(&m.funcs, "  %%t%d = call i32 @%s()\n", m.tempID, c.Fn)
	m.tempID++
}

func (m *Module) emitRet(r *hir.Ret) {
	if r.Val == nil {
		wprintf(&m.funcs, "  ret i32 0\n")
		return
	}
	switch v := r.Val.(type) {
	case hir.ConstInt:
		wprintf(&m.funcs, "  ret i32 %s\n", v.Text)
	default:
		wprintf(&m.funcs, "  ret i32 0\n")
	}
}

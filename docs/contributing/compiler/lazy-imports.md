# Lazy Import Initialization

**Scope:** Compiler implementation for deferred module initialization. Modules are initialized only when their symbols are first accessed, not at program start.

---

## Overview

By default, all imported modules are **lazy**: their `__top__()` initializer is called on first symbol access rather than at program start. This provides:

- **Automatic circular dependency resolution** — modules can import each other without initialization ordering issues
- **Performance gains** — unused imports never initialize
- **Zero developer effort** — no syntax changes required

---

## Implementation Architecture

### Phase 1: Resolver Tracking

**File:** `compiler/internal/resolve/resolve.go`

The resolver builds a `LazyModules` set during import resolution:

```go
type Resolver struct {
    LazyModules map[string]bool  // module path → needs lazy init
}
```

All imported modules are marked lazy. The set is passed to the LLVM backend.

### Phase 2: Init Thunk Generation

**File:** `compiler/internal/backend/llvm/module.go`

For each lazy module, the backend generates:

1. **Init flag** — global boolean tracking initialization state:
   ```llvm
   @__mod_math_init_done = internal global i1 false
   ```

2. **Init thunk** — function that checks flag and calls `__top__()`:
   ```llvm
   define void @__ensure_math_init() {
   entry:
     %done = load i1, ptr @__mod_math_init_done
     br i1 %done, label %skip, label %init
   init:
     call i32 @math___top__()
     store i1 true, ptr @__mod_math_init_done
     br label %skip
   skip:
     ret void
   }
   ```

### Phase 3: Per-Call Init Injection

**Files:** 
- `compiler/internal/backend/llvm/module.go` — `ensureLazyModuleInit()`
- `compiler/internal/backend/llvm/emit_call.go` — integration point

When emitting any function call, the backend checks if the callee belongs to a lazy module:

```go
func (m *Module) ensureLazyModuleInit(fnName string) bool {
    for modPath := range m.lazyModules {
        safeName := sanitizeModName(modPath)
        prefix := safeName + "_"
        
        if strings.HasPrefix(fnName, prefix) {
            thunkName := fmt.Sprintf("__ensure_%s_init", safeName)
            if !m.lazyInitEmitted[modPath+"_called"] {
                m.lazyInitEmitted[modPath+"_called"] = true
                wprintf(&m.funcs, "  call void @%s()\n", thunkName)
            }
            return true
        }
    }
    return false
}
```

This emits the thunk call **once** before the first call to any function from that module.

---

## Key Design Decisions

### Why Not Declare Before Define?

LLVM does not allow `declare` followed by `define` for the same function. The thunk is defined in `emitLazyInitThunks()`, so we cannot emit a declaration in `ensureLazyModuleInit()`. The thunk definition appears later in the IR, and LLVM handles forward references correctly.

### Function Name Prefixing

Module functions are prefixed with the sanitized module name (e.g., `math.sin()` becomes `math_sin`). The `ensureLazyModuleInit()` checks for this prefix to identify module membership.

### Init Once Per Module

The `lazyInitEmitted[modPath+"_called"]` flag ensures each module's init thunk is called only once, regardless of how many functions from that module are called.

---

## Diagnostics

- **DMW0004**: Unused import warnings still work — lazy imports that are never accessed still produce warnings at compile time.

---

## Testing

```bash
# Single test
./test_examples.sh 329,329

# Full suite
./test_examples.sh
```

Example: `examples/329_lazy_import.desi`

---

## Future Work

- `from X import Y` syntax — currently the init happens on module-qualified calls; direct imports may need additional handling
- Module field access (e.g., `mod.CONSTANT`) — constants don't go through `emitCall()`
- Eager opt-in — allow `import! math` syntax to force eager initialization

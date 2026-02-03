# Recursion Limit Implementation

**Scope:** Runtime recursion depth tracking to prevent stack overflow.

---

## Overview

Desi enforces a configurable maximum recursion depth (default: 1000). Exceeding this limit triggers a runtime panic with a clear error message.

---

## Implementation

### Runtime (`compiler/runtime/limits.c`)

Thread-local call depth counter:

```c
static __thread int64_t __call_depth = 0;
static int64_t __max_recursion = 1000;

void __desi_call_enter(void) {
    __call_depth++;
    if (__call_depth > __max_recursion) {
        fprintf(stderr, "Desi panic: maximum recursion depth exceeded (%lld)\n", 
                (long long)__max_recursion);
        exit(1);
    }
}

void __desi_call_exit(void) {
    __call_depth--;
}
```

### LLVM Backend Instrumentation

#### Function Entry (`emit_func.go`)

For user functions (excluding `main` and `__top__`):

```go
if firstBlock && fn.Name != "main" && !strings.HasSuffix(fn.Name, "__top__") {
    m.ensureDecl("declare void @__desi_call_enter()")
    wprintf(&m.funcs, "  call void @__desi_call_enter()\n")
}
```

#### Function Exit (`module.go`)

Before each `ret` instruction:

```go
if m.curFuncName != "" && m.curFuncName != "main" && 
   !strings.HasSuffix(m.curFuncName, "__top__") {
    m.ensureDecl("declare void @__desi_call_exit()")
    wprintf(&m.funcs, "  call void @__desi_call_exit()\n")
}
```

---

## Testing

```bash
./test_examples.sh 330,330  # Runs recursion limit test
```

Example output:
```
Desi panic: maximum recursion depth exceeded (1000)
```

---

## Future Work

- CLI flag `--max-recursion=N` to configure limit
- `__desi_set_max_recursion(N)` runtime API

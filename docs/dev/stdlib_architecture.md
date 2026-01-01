# Stdlib Architecture: Hybrid C + Desi Pattern

This document describes Desi's approach to building standard library modules that achieve **C-like performance** with **memory safety**.

## Philosophy

Desi prioritizes three goals equally:
1. **Performance**: Match C/C++/Rust speed
2. **Safety**: Rust-like memory safety without garbage collection
3. **Ergonomics**: Python-like syntax and ease of use

To achieve all three, we use a **hybrid architecture** where:
- **Performance-critical code** (parsing, byte manipulation) → C runtime
- **User-facing APIs** (safe types, error handling) → Pure Desi

## The Hybrid Pattern

```
┌─────────────────────────────────────────────────┐
│           User Code (100% Desi)                 │
│   import json                                   │
│   let data = json.parse(text)                   │
└─────────────────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────┐
│     compiler/lib/<module>.desi (Desi API)       │
│   • Public API functions                        │
│   • Safe wrapper types (enums, classes)         │
│   • Error handling with Result/Option           │
│   • Extern bindings to C runtime                │
└─────────────────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────┐
│   compiler/runtime/<module>.c (C Runtime)       │
│   • Byte-level operations                       │
│   • Performance-critical algorithms             │
│   • Zero-copy optimizations                     │
│   • SIMD where applicable                       │
└─────────────────────────────────────────────────┘
```

## When to Use C vs Desi

| Use C Runtime When... | Use Pure Desi When... |
|-----------------------|-----------------------|
| Byte-by-byte parsing (lexers) | User-facing API design |
| Memory layout matters | Type-safe wrappers |
| SIMD/vectorization helps | Error handling (Result/Option) |
| Existing C library available | Business logic |
| Hot loops (>1M iterations) | Convenience methods |

## Example: JSON Module

### C Runtime (`compiler/runtime/json.c`)
```c
// Fast tokenizer - this is the hot path
typedef struct {
    const char* input;
    size_t pos;
    size_t len;
} JsonLexer;

JsonToken* __json_next_token(JsonLexer* lexer) {
    // Byte-by-byte parsing - C is 10-50x faster than
    // safe string iteration for this specific task
    while (lexer->pos < lexer->len) {
        char c = lexer->input[lexer->pos++];
        // ... fast token extraction
    }
}

// Recursive descent parser - stack-efficient
JsonNode* __json_parse_value(JsonLexer* lexer) {
    // Returns raw pointer, wrapped by Desi layer
}
```

### Desi API (`compiler/lib/json.desi`)
```desi
# Safe user-facing type
pub enum JsonValue:
    Null
    Bool(bool)
    Number(float)
    String(str)
    Array(list[JsonValue])
    Object(dict[str, JsonValue])

# Extern bindings (internal, not exposed to users)
extern def __json_parse_raw(text: str) -> ptr
extern def __json_get_error() -> str

# Safe public API
pub def parse(text: str) -> Result[JsonValue, JsonError]:
    let raw = __json_parse_raw(text)
    if raw == null:
        Result.Err(JsonError(__json_get_error()))
    else:
        Result.Ok(__wrap_value(raw))

# Safe accessors
pub def get(self: JsonValue, key: str) -> Option[JsonValue]:
    match self:
        case Object(d): d.get(key)
        case _: Option.None
```

## Import Requirement

All stdlib modules require explicit import:
```desi
import json      # Required before json.parse()
import http      # Required before http.get()
import crypto    # Required before crypto.hash()
```

This enables **dead-code elimination** - unused modules are not linked.

## Performance Expectations

With the hybrid approach:

| vs C/cJSON | Overhead | Reason |
|------------|----------|--------|
| Parsing | ~0% | Same C code |
| Wrapping | <5% | One-time conversion |
| Accessors | ~10% | Bounds checking (safety) |
| Overall | <10% | Rust-like performance |

## Adding New Stdlib Modules

1. **Identify hot paths** - What needs C speed?
2. **Design safe Desi API** - User-facing types and functions
3. **Implement C runtime** - Put in `compiler/runtime/<module>.c`
4. **Create Desi wrapper** - Put in `compiler/lib/<module>.desi`
5. **Add to stdlib tracking** - Update `check/stmt.go` StdlibImports
6. **Write tests** - Both success and expected-error cases
7. **Document** - Add to `book/docs/language/builtins.md`

## Future Optimization Path

As the Desi compiler improves:
1. Pure Desi implementations may match C speed
2. C runtime becomes optional optimization
3. Self-hosting increases (Desi-in-Desi)

The hybrid architecture allows **gradual migration** without breaking user code.

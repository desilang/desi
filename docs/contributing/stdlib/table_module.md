# Table Module Implementation

Internal documentation for the `table` standard library module.

## Architecture

```
compiler/lib/table.desi     → Desi API (create, add rows, render)
compiler/runtime/table.c    → C runtime (column layout, ASCII/Unicode rendering)
```

## C Runtime Symbols

| C Symbol | Desi API | Signature |
|----------|----------|-----------|
| `__table_new` | `table.new` | `(DesiList*, int32) → DesiTable*` |
| `__table_add_row` | `table.add_row` | `(DesiTable*, DesiList*, int32) → int32` |
| `__table_set_align` | `table.set_align` | `(DesiTable*, int32, char) → int32` |
| `__table_len` | `table.size` | `(DesiTable*) → int32` |
| `__table_render` | `table.render` | `(DesiTable*) → char*` |
| `__table_render_simple` | `table.render_simple` | `(DesiTable*) → char*` |
| `__table_free` | `table.free` | `(DesiTable*) → void` |

## ABI Notes — DesiList*

**Critical**: `__table_new` and `__table_add_row` accept `DesiList*` (not `const char**`).

When Desi passes `list<str>` to a C extern function, the compiler passes a pointer to a `DesiList` struct:

```c
typedef struct {
    void** data;       // Array of char* pointers
    size_t length;
    size_t capacity;
    int type_tag;      // 1 = str
    ElemToStrFunc to_str_fn;
} DesiList;
```

The C runtime accesses elements as `(const char*)list->data[i]` and uses `list->length` for bounds.

**This was a bug fix**: the original implementation accepted `const char**` which caused segfaults since the compiler passes `DesiList*`.

## Implementation Notes

- Column widths are computed automatically based on header and cell content lengths.
- `render()` uses UTF-8 box-drawing characters (┌ ─ ┐ │ ├ ┼ ┤ └ ┴ ┘).
- `render_simple()` uses ASCII characters (`+`, `-`, `|`).
- The Desi API hides the `ncols` parameter — it's inferred from the list length.
- Handles are `cptr` — opaque pointers to `DesiTable` structs.

## Test Coverage

- `examples/513_table_module.desi` — create, add_row, size, render_simple

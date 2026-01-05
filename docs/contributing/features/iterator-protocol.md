# Iterator Protocol Implementation

This document provides a comprehensive technical overview of Desi's lazy iterator protocol for compiler contributors.

## Architecture Overview

The iterator protocol spans four main components:

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│ Type System     │ ──► │ Type Checker    │ ──► │ HIR Lowering    │ ──► │ C Runtime       │
│ (types.go)      │     │ (expr_field.go) │     │ (lower/*.go)    │     │ (iterator.c/h)  │
└─────────────────┘     └─────────────────┘     └─────────────────┘     └─────────────────┘
```

## Type System (`compiler/internal/types/types.go`)

### Iterator Types

Three iterator types are defined:

```go
// ListIter represents an iterator over a list
type ListIter struct {
    Elem T  // Element type
}

// MapIter represents a mapping iterator (for future .map() support)
type MapIter struct {
    Source T  // Source iterator type
    Elem   T  // Output element type
}

// FilterIter represents a filtering iterator (for future .filter() support)
type FilterIter struct {
    Source T  // Source iterator type
    Elem   T  // Element type (same as source)
}
```

Each implements the `T` interface:
- `isType()` - Marker method
- `String()` - Returns `"ListIter[<elem>]"`, etc.

## Type Checking (`compiler/internal/check/expr_field.go`)

### Method Resolution

The type checker resolves iterator methods in `typFieldExpr`:

```go
switch t := t.(type) {
case *types.List:
    return c.resolveListMethod(x, t)
case *types.ListIter:
    return c.resolveListIterMethod(x, t)
case *types.MapIter:
    return c.resolveMapIterMethod(x, t)
case *types.FilterIter:
    return c.resolveFilterIterMethod(x, t)
// ...
}
```

### `resolveListMethod` - List Methods

Handles `.iter()` on lists:

```go
case "iter":
    // iter() -> ListIter[T]
    methodType = types.FuncOf(nil, &types.ListIter{Elem: l.Elem}, false)
```

### `resolveListIterMethod` - Iterator Methods

Handles methods on `ListIter`:

```go
func (c *checker) resolveListIterMethod(x *ast.FieldExpr, li *types.ListIter) types.T {
    switch x.Name.Name {
    case "collect":
        // collect() -> list[T]
        methodType = types.FuncOf(nil, &types.List{Elem: li.Elem}, false)
    case "first":
        // first() -> Option[T]
        methodType = types.FuncOf(nil, types.OptionOf(li.Elem), false)
    case "map":
        // map(fn: T -> U) -> MapIter[T, U]
        // (extracts U from function return type)
    case "filter":
        // filter(predicate: T -> bool) -> FilterIter[T]
    }
}
```

## HIR Lowering

### List Method Lowering (`compiler/internal/lower/list_lower.go`)

The `.iter()` method allocates a stack iterator:

```go
case "iter":
    // iter() -> ListIter (stack-allocated)
    // ListIter in C = {DesiList* list, int64_t index} = {ptr, i64}
    iterPtr := ls.b.FreshTemp("iter_ptr")
    ls.b.Emit(&hir.Alloca{Dst: iterPtr, Type: "{ptr, i64}", Count: 1})
    ls.b.Emit(&hir.Call{Fn: "list_iter_init", Args: []hir.Value{iterPtr, receiver}})
    return iterPtr
```

**Key design decision**: Stack allocation (`{ptr, i64}`) avoids heap pressure.

### Iterator Method Lowering (`compiler/internal/lower/iterator_lower.go`)

```go
func (ls *lowerState) lowerListIterMethod(fe *ast.FieldExpr, args []ast.Expr, iterType *types.ListIter) hir.Value {
    receiver := ls.lowerExpr(fe.X)
    methodName := fe.Name.Name

    switch methodName {
    case "collect":
        // collect() -> list[T]
        result := ls.b.FreshTemp("collected")
        typeTag := ls.getIterTypeTag(iterType.Elem)
        ls.b.Emit(&hir.Call{Dst: result, Fn: "list_iter_collect", Args: []hir.Value{receiver, typeTag}, Type: "ptr"})
        return result

    case "first":
        // first() -> Option[T]
        elem := ls.b.FreshTemp("first_elem")
        ls.b.Emit(&hir.Call{Dst: elem, Fn: "list_iter_next", Args: []hir.Value{receiver}, Type: "ptr"})
        return elem
    }
}
```

### Dispatch Location (`compiler/internal/lower/lower_call.go`)

Type dispatch added around line 127:

```go
// ListIter methods: collect, map, filter, first
if t, ok := feXType.(*types.ListIter); ok {
    return ls.lowerListIterMethod(fe, x.Args, t)
}
```

## C Runtime (`compiler/runtime/iterator.c`, `iterator.h`)

### Data Structures

```c
// Stack-allocated iterator over a list
typedef struct {
    DesiList* list;   // Pointer to source list (not owned)
    int64_t index;    // Current position
} ListIter;

// Mapping iterator (stack-allocated)
typedef struct {
    void* source;            // Source iterator (any type)
    void* (*map_fn)(void*);  // Transformation function
} MapIter;

// Filtering iterator (stack-allocated)
typedef struct {
    void* source;             // Source iterator
    bool (*pred_fn)(void*);   // Predicate function
} FilterIter;
```

### Core Functions

```c
// Initialize a pre-allocated ListIter
void list_iter_init(ListIter* iter, DesiList* list);

// Get next element (NULL when exhausted)
void* list_iter_next(ListIter* iter);

// Collect iterator into new list
DesiList* list_iter_collect(ListIter* iter, int type_tag);
```

### Implementation Details

**`list_iter_init`**:
```c
void list_iter_init(ListIter* iter, DesiList* list) {
    iter->list = list;
    iter->index = 0;
}
```

**`list_iter_next`**:
```c
void* list_iter_next(ListIter* iter) {
    if (iter->index >= iter->list->length) {
        return NULL;  // Iterator exhausted
    }
    void* elem = iter->list->data[iter->index];
    iter->index++;
    return elem;
}
```

**`list_iter_collect`**:
```c
DesiList* list_iter_collect(ListIter* iter, int type_tag) {
    DesiList* result = list_new(type_tag, NULL);
    
    while (true) {
        void* elem = list_iter_next(iter);
        if (!elem) break;
        list_append(result, elem, type_tag);
    }
    
    return result;
}
```

## Class `__new__` Integration

### Problem

When `__new__` contains `return ClassName(field=val)`, the naive lowering would:
1. Allocate new instance
2. Call `ClassName___new__` (itself!) → **Infinite recursion**

### Solution

Added `dunderNew` context tracking to `lowerState`:

```go
type lowerState struct {
    // ...existing fields...
    
    inDunderNew     bool      // true when lowering inside a __new__ method body
    dunderNewClass  string    // class name for the current __new__
    dunderNewSelf   hir.Value // the self pointer to initialize
}
```

**`LowerFuncForDunderNew`** (`hir_lower.go`):
```go
func LowerFuncForDunderNew(fd *ast.FuncDecl, info *check.Info, src []byte, className string, selfPtr hir.Value) *hir.Func {
    return lowerFuncFromDeclWithContext(fd, info, src, className, selfPtr)
}
```

**Detection in `lower_call.go`**:
```go
if isConstructor {
    // Inside __new__, ClassName(field=val) initializes self, not new allocation
    if ls.inDunderNew && cls.Name == ls.dunderNewClass {
        // Initialize self's fields directly
        for i, arg := range x.Args {
            // ... emit GEP and Store for each field ...
        }
        return nil  // void return for __new__
    }
    
    // Normal case: allocate and call __new__
    // ...
}
```

## Generated LLVM IR

### Iterator Creation and Collection

```llvm
; Create iterator (stack-allocated)
%iter_ptr7 = alloca {ptr, i64}
%t0 = call i32 @list_iter_init(ptr %iter_ptr7, ptr %list1)

; Collect to new list
%collected8 = call ptr @list_iter_collect(ptr %iter_ptr7, i32 0)
```

### Class __new__ (Fixed)

```llvm
; Wrapper: allocates and calls __new__
define ptr @Simple(i32 %v) {
entry:
  %instance = call ptr @malloc(i32 4)
  call void @Simple___new__(ptr %instance, i32 %v)
  ret ptr %instance
}

; User's __new__: initializes self's fields
define void @Simple___new__(ptr %self, i32 %v) {
entry:
  %field_ptr = getelementptr inbounds i8, ptr %self, i32 0
  store i32 %v, ptr %field_ptr
  ret void
}
```

## Testing Verified Edge Cases

| Test | Description | Status |
|------|-------------|--------|
| Empty list | `[].iter().collect()` | ✅ |
| Single element | `[42].iter().collect()` | ✅ |
| Multiple iterations | Same source, multiple iterators | ✅ |
| Source preservation | Original unchanged after iteration | ✅ |
| String generics | `list[str]` iteration | ✅ |
| Struct fields | Field values preserved | ✅ |
| Class instances | Class field access after collection | ✅ |

## File Reference

| File | Purpose |
|------|---------|
| `compiler/internal/types/types.go` | Iterator type definitions |
| `compiler/internal/check/expr_field.go` | Method resolution (lines 287-310, 925-1020) |
| `compiler/internal/lower/list_lower.go` | `.iter()` lowering |
| `compiler/internal/lower/iterator_lower.go` | `.collect()`, `.first()` lowering |
| `compiler/internal/lower/lower_call.go` | Type dispatch (line 127), __new__ context |
| `compiler/internal/lower/hir_lower.go` | `LowerFuncForDunderNew`, lowerState fields |
| `compiler/internal/lower/class_lower.go` | Class constructor generation |
| `compiler/runtime/iterator.c` | C runtime implementation |
| `compiler/runtime/iterator.h` | C runtime header |

## Future Work

1. **Map/Filter Adaptors**: Implement lazy `map()` and `filter()` with closure support
2. **Range Iterators**: `range(start, end).iter()`
3. **For-Loop Integration**: `for x in list.iter()` syntax
4. **Iterator Trait**: Generic `Iterator<T>` trait for custom types

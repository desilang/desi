# Iterator Protocol Implementation (`__iter__` / `__next__`)

This document describes the implementation of custom iterators for `for` loops.

## Overview

Classes can implement the iterator protocol with two dunders:
- `__iter__(self) -> Iterator` - Returns the iterator object (often `self`)
- `__next__(self) -> Option<T>` - Returns next element or `Nothing` when exhausted

## Type Checking (check/class.go)

1. Dunders are registered in `cls.Dunders` map during class analysis
2. `__iter__` must return a class type
3. `__next__` must return `Option<T>` for some element type `T`

## Lowering (lower/lower_stmt.go)

In `ForStmt` handling (around line 1375):

```go
// Check for __iter__/__next__ iterator protocol
if cls, ok := ls.info.Types[s.Iter].(*types.Class); ok {
    _, hasIter := cls.Dunders["__iter__"]
    nextDunder, hasNext := cls.Dunders["__next__"]
    if hasIter && hasNext {
        // Extract element type from Option<T>
        elemType := types.OptionSomeType(nextDunder.Ret)
        elemLLVMType := lowerType(elemType)
        
        // Generate: iter = obj.__iter__()
        // Loop:
        //   option = iter.__next__()
        //   if option.tag == 0 (Some): extract value, run body
        //   else: exit loop
    }
}
```

### Generated LLVM Pattern

```llvm
; Call __iter__
%iter_obj = call ptr @ClassName___iter__(ptr %obj)
br label %for_iter_cond

for_iter_cond:
  ; Call __next__
  %option = call ptr @ClassName___next__(ptr %iter_obj)
  
  ; Check Option tag (0 = Some, 1 = Nothing)
  %tag_ptr = getelementptr inbounds i8, ptr %option, i32 0
  %tag = load i32, ptr %tag_ptr
  %has_value = icmp eq i32 %tag, 0
  br i1 %has_value, label %for_iter_body, label %loop_exit

for_iter_body:
  ; Extract payload pointer (at offset 4)
  %payload_ptr = getelementptr inbounds i8, ptr %option, i32 4
  %value_ptr = load ptr, ptr %payload_ptr
  %value = load <elemType>, ptr %value_ptr
  ; ... loop body ...
  br label %for_iter_cond

loop_exit:
  ; ... post-loop code ...
```

## Option Memory Layout

```
Option { tag: i32, payload: ptr }
- offset 0: tag (0 = Some, 1 = Nothing)
- offset 4: payload pointer (holds actual value)
```

## Generic Type Handling

The element type is extracted at lowering time:
```go
if innerType := types.OptionSomeType(nextDunder.Ret); innerType != nil {
    elemType = innerType
    elemLLVMType = lowerType(innerType)
}
```

This ensures `for x in IntIter:` gives `x: int` while `for s in StrIter:` gives `s: str`.

## Priority Order

Iterator detection occurs AFTER `__getitem__`/`__len__` check but BEFORE list fallback:
1. Tuple iteration (compile-time unrolled)
2. enumerate(), reversed(), zip() builtins
3. `__getitem__` + `__len__` (index-based iteration)
4. **`__iter__` + `__next__` (iterator protocol)** ← NEW
5. List iteration (default)

## Key Implementation Points

1. The `return` after While emission is **required** to prevent fall-through to list iteration
2. Option field access uses `i8` byte-offset GEP, not `%Option` type (undeclared)
3. Element type is determined from `__next__` return type at lowering time

## Test Coverage

- `examples/249_iter_dunder.desi` - Range iterator class

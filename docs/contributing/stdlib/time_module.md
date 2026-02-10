# Time Module - Contributor Guide

Implementation details for the `time` module in `compiler/lib/time/__mod.desi`.

## Architecture

The time module provides time/date functionality through:

1. **C Runtime Functions** (`compiler/runtime/time.c`) - Low-level FFI bindings
2. **Desi Wrappers** (`compiler/lib/time/__mod.desi`) - Safe, ergonomic API

## Key Components

### Duration Class

Represents time spans. Has mutable `seconds` field for factory methods.

```desi
pub class Duration:
    pub mut seconds: float  # Must be 'mut' for factory methods
    
    @staticmethod
    pub def from_seconds(s: float) -> Duration:
        let d = Duration()
        d.seconds := s  # Use := for mutation
        return d
```

### Stopwatch Class

Timing utility with RAII support.

```desi
pub class Stopwatch:
    pub mut start_time: float
    pub mut silent_mode: bool
    
    pub def __close__(self) -> none:
        # Called at scope exit when used with 'using'
```

### Factory Functions

```desi
pub def stopwatch() -> Stopwatch      # Auto-starts, prints on close
pub def stopwatch_silent() -> Stopwatch  # No print on close
pub def create_duration(seconds: float) -> Duration
```

## Export Resolution Fix (Feb 2026)

### Problem

Functions returning class types weren't being exported. `CollectExports()` in `exports.go` used `types.FromName()` which only recognizes built-in types.

### Solution

Modified `CollectExports()` to:

1. Pre-collect local class names in first pass
2. Use `resolveType()` helper that checks both builtins AND local classes

```go
// First pass: collect class names
localClasses := make(map[string]*types.Class)
for _, d := range mod.Decls {
    if cls, ok := d.(*ast.ClassDecl); ok {
        localClasses[cls.Name.Name] = ...
    }
}

// Helper to resolve type from name
resolveType := func(name string) (types.T, bool) {
    if t, ok := types.FromName(name); ok { return t, true }
    if cls, ok := localClasses[name]; ok { return cls, true }
    return nil, false
}
```

### Known Limitation

Methods on factory-returned class types don't resolve:

```desi
stopwatch().elapsed()  # ❌ Error
Stopwatch().elapsed()  # ✅ Works
```

The returned class type placeholder from exports doesn't include method information. Fix requires propagating full class type info through imports.

## Indentation Requirements

Desi requires **tabs** for indentation. The `time/__mod.desi` file was converted using:

```bash
unexpand -t 4 file.desi > file.tabs && mv file.tabs file.desi
```

## Testing

Run time module tests:
```bash
./test_examples.sh 400,400
```

## Related Files

- [exports.go](file:///Users/desiprogrammer/Desktop/Projects/go_stuff/desi/compiler/internal/resolve/exports.go) - Export collection
- [import_sigs.go](file:///Users/desiprogrammer/Desktop/Projects/go_stuff/desi/compiler/internal/check/import_sigs.go) - Import signature population
- [lower_call.go](file:///Users/desiprogrammer/Desktop/Projects/go_stuff/desi/compiler/internal/lower/lower_call.go) - Duration return type workaround

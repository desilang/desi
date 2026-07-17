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

## Cross-Module Method Resolution Fix (Feb 2026)

### Problem

Two related issues in `CollectExports()`:
1. Functions returning class types weren't exported (`types.FromName()` only handles builtins)
2. Methods on factory-returned classes didn't resolve (placeholder class had empty `Methods` map)

### Solution

Restructured `CollectExports()` in `exports.go`:

1. **Collect classes FIRST** via `collectClasses()` function
2. **Use `out.Classes`** in `resolveType()` so function return types reference the same class instance

```go
func collectClasses(mod *ast.Module, out *Exports) {
    // Creates class with Methods, Dunders, Properties populated
    classType := &types.Class{
        Methods:    map[string]*types.Func{},
        Dunders:    map[string]*types.Func{},
        Properties: map[string]*types.Func{},
    }
    // ... populates all methods ...
    out.Classes[name] = classType
}

func CollectExports(mod *ast.Module) *Exports {
    collectClasses(mod, out)  // FIRST: collect classes with full method info
    
    resolveType := func(name string) (types.T, bool) {
        if cls, ok := out.Classes[name]; ok {
            return cls, true  // Same instance as used for method lookup!
        }
        // ...
    }
    // SECOND: collect functions using resolveType
}
```

### Result

Both patterns now work:
```desi
stopwatch().elapsed()     # ✅ Factory + method
Stopwatch().elapsed()     # ✅ Constructor + method
```

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

- [exports.go](../../../compiler/internal/resolve/exports.go) - Export collection
- [import_sigs.go](../../../compiler/internal/check/import_sigs.go) - Import signature population
- [lower_call.go](../../../compiler/internal/lower/lower_call.go) - Duration return type workaround

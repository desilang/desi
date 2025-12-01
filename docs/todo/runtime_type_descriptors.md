# TODO: Runtime Type Descriptors

**Status**: Under Discussion  
**Priority**: Low  
**Related**: Generics, Reflection, Error handling

## Overview

Runtime type descriptors provide type information at runtime, enabling reflection, better error messages, and dynamic behavior.

## Pros and Cons

### ✅ Pros

1. **Better Error Messages**
```desi
# Without descriptors:
panic("unwrap failed")

# With descriptors:
panic("unwrap failed for Option<User>")  # Shows actual type!
```

2. **Generic Debugging**
```desi
def debug<T>(value: T):
    # Without descriptors:
    print("Debug: ???")  # Can't show type
    
    # With descriptors:
    print(f"Debug ({type_name<T>()}): {value}")  # Outputs: "Debug (int): 42"
```

3. **Runtime Type Checking**
```desi
def process(value: Any) -> Result<Data>:
    # Can check actual type at runtime
    if runtime_type(value) == type_of<Data>():
        return Ok(value as Data)
    else:
        return Err("expected Data, got {runtime_type_name(value)}")
```

4. **Serialization/Deserialization**
```desi
def to_json<T>(value: T) -> String:
    # Can inspect T's structure at runtime
    match runtime_type<T>():
        Struct(fields): 
            # Serialize each field
        Enum(variants):
            # Serialize variant
```

5. **Dynamic Programming**
```desi
# Create instances from type names
let typ = type_from_name("Box<int>")
let instance = typ.construct(42)
```

### ❌ Cons

1. **Binary Size Increase**
   - Every generic type needs descriptor
   - Type names stored as strings
   - +10-20% binary size increase

2. **Runtime Overhead**
   - Extra pointer in each generic value
   - Cache miss on descriptor access
   - ~5% performance hit

3. **Complexity**
   - More compiler infrastructure
   - Maintenance burden
   - More testing needed

4. **Conflicts with Trait System**
```desi
# Better approach: Use traits instead of descriptors
trait Debug:
    def debug_string(self) -> String

def debug<T: Debug>(value: T):
    print(value.debug_string())  # No descriptor needed!
```

## Alternative: Trait-Based Approach (Recommended)

Instead of runtime descriptors, use **compile-time trait bounds**:

```desi
# Instead of:
def print_any<T>(val: T):
    print(type_name<T>(), val)  # Needs descriptor

# Use traits:
trait Display:
    def to_string(self) -> String

def print_any<T: Display>(val: T):
    print(val.to_string())  # No descriptor needed!

# Compiler ensures T implements Display
```

### Benefits of Trait Approach

✅ No runtime overhead  
✅ Smaller binaries  
✅ Compile-time safety  
✅ Explicit capabilities  
✅ Better performance  

## When Descriptors Are Actually Needed

1. **Reflection Libraries**
   - ORM (Object-Relational Mapping)
   - Serialization frameworks
   - Debugging tools

2. **Dynamic Languages Interop**
   - Python FFI
   - JavaScript FFI
   - Need to map Desi types to dynamic types

3. **Advanced Error Reporting**
   - Showing full type in panic messages
   - Stack trace with type information

## Proposed Design (If Implemented)

### Minimal Descriptor

```go
// In runtime
struct TypeDescriptor {
    name: *const char,      // "Box<int>"
    size: usize,            // sizeof(T)
    align: usize,           // alignof(T)
    kind: TypeKind,         // Struct, Enum, Primitive, etc.
};

enum TypeKind {
    Primitive,
    Struct,
    Enum,
    Tuple,
    Function,
}
```

### Compiler Generation

```go
// For each instantiated generic type, generate descriptor
// e.g., for Box<int>:

@type_desc_Box_int = global {
    i8* @"Box<int>",  // name
    i64 8,             // size
    i64 8,             // align
    i32 1,             // kind = Struct
}

// Generic function stores descriptor pointer
define ptr @identity(ptr %x, ptr %type_desc) {
    // Can query type_desc if needed
    ret ptr %x
}
```

### User API

```desi
# Get type information
def print_type_info<T>(val: T):
    print(f"Type: {type_name<T>()}")
    print(f"Size: {type_size<T>()} bytes")
    print(f"Align: {type_align<T>()} bytes")

# Runtime type checking
def is_same_type<T, U>(a: T, b: U) -> bool:
    return type_id<T>() == type_id<U>()

# Create from type name
def dynamic_create(type_name: str, value: Any) -> Option<Any>:
    match type_from_name(type_name):
        Some(typ): return Some(typ.construct(value))
        None: return None
```

## Implementation Complexity

### Low Priority Because:

1. **Can wait until we have more use cases**
2. **Trait system provides most benefits**
3. **Can add later without breaking changes**
4. **Significant implementation effort for marginal benefit**

### If Implemented:

- [ ] Design minimal descriptor format
- [ ] Add descriptor generation to codegen
- [ ] Pass descriptors to generic functions (ABI change)
- [ ] Implement intrinsics: `type_name`, `type_size`, etc.
- [ ] Add reflection API
- [ ] Benchmark performance impact
- [ ] Document trade-offs for users

## Recommendation

**Don't implement now.** Instead:

1. ✅ Implement trait system first
2. ✅ Use trait bounds for capabilities
3. ✅ Wait for concrete use cases that require descriptors
4. ✅ Re-evaluate when we have ORM/serialization libraries

## Example: Trait-Based vs Descriptor-Based

### Scenario: Generic Printing

```desi
# ❌ Descriptor approach (not recommended)
def print<T>(val: T):
    # Requires runtime descriptor
    if type_kind<T>() == TypeKind.Primitive:
        print_primitive(val)
    elif type_kind<T>() == TypeKind.Struct:
        for field in type_fields<T>():
            print(f"{field.name}: {field.get(val)}")

# ✅ Trait approach (recommended)
trait Display:
    def to_string(self) -> String

def print<T: Display>(val: T):
    # Compile-time guarantee T has to_string()
    print(val.to_string())

# User implements trait
impl Display for User:
    def to_string(self) -> String:
        return f"User({self.name})"
```

**Trait approach is better**: Faster, smaller, type-safe!

## Decision Criteria

Implement descriptors only if we encounter a use case where:
1. Traits cannot solve the problem
2. The benefit outweighs the cost
3. Users actually need the feature

**Current verdict**: Wait and see. Focus on traits first.

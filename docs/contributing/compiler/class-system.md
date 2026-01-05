# Desi Class System - Internals & Implementation Guide

> **For Contributors**: This document covers the complete class system design, implementation details, and future roadmap.

---

## Table of Contents

1. [Design Philosophy](#design-philosophy)
2. [Current Implementation Status](#current-implementation-status)
3. [Class Visibility Model](#class-visibility-model)
4. [Nested Classes](#nested-classes)
5. [Memory Management](#memory-management)
6. [Friend Access Pattern](#friend-access-pattern)
7. [Implementation Files](#implementation-files)
8. [Phased Implementation Roadmap](#phased-implementation-roadmap)

---

## Design Philosophy

### Why These Design Choices?

Desi's class system is designed to be **Python-familiar with systems-level control**. Here's the rationale for each major decision:

### 1. Top-Level Classes: Public by Default

**Decision**: `class Foo:` = public (no `pub` keyword needed)

**Rationale**:
- **Python familiarity**: Python developers expect classes to be usable immediately
- **Reduced boilerplate**: Top-level things are typically meant to be exported
- **Explicit private**: If you want private, make it nested or use module-level convention

**Comparison**:
- Python: Public by default ✓
- Rust: Private by default (file-level visibility)
- Java: Package-private by default

### 2. Nested Classes: Private by Default

**Decision**: `class Outer: class Inner:` = private

**Rationale**:
- **Encapsulation**: Nested classes are implementation details
- **Explicit export**: Use `pub class Inner:` to explicitly expose
- **Django Meta pattern**: `class Model: class Meta:` is private by design

**Comparison**:
- Python: No true private, relies on convention (`_ClassName`)
- Java: Inner class visibility matches declaration
- C++: Private by default (same as Desi)

### 3. Static Nesting Only (No Implicit Outer Reference)

**Decision**: Nested classes don't hold implicit reference to outer

**Rationale**:
- **Memory predictability**: No hidden pointer storage
- **Lifetime clarity**: Nested instance can outlive outer
- **Explicit is better**: If you need outer, pass it explicitly
- **Simpler compilation**: No closure-like capture mechanics

**Comparison**:
- Python: Has implicit outer access (via closure over `self`)
- Java: Non-static inner classes have implicit `Outer.this`
- Rust: No implicit capture (same as Desi)
- C++: Static nested by default (same as Desi)

### 4. Family Access Rule

**Decision**: Nested classes can access outer's private members

**Rationale**:
- **Internal helpers**: Iterator classes need collection internals
- **Encapsulation preserved**: External code still can't access privates
- **Test-friendly**: Nested test helpers can access internals
- **Builder pattern**: `Request.Builder` needs private constructor

**Example use case**:
```python
class LinkedList:
    _head: Node
    
    class Iterator:
        pub def next(self, list: LinkedList) -> int:
            return list._head.value  # ✅ Allowed - family access
```

### 5. Four Memory Levels

**Decision**: Offer gradual safety from manual → arena → Rc → Box

**Rationale**:
- **Performance choice**: Not everyone needs RC overhead
- **Embedded/systems**: Some contexts need manual control
- **Web/apps**: Rc provides convenience without GC pauses
- **Safety-critical**: Box/ownership prevents entire classes of bugs

**Why not just GC?**
- **Latency**: GC pauses are unacceptable in real-time contexts
- **Memory footprint**: GC has overhead
- **Control**: Systems programmers need deterministic cleanup
- **FFI**: Interfacing with C requires predictable memory

### 6. Friend Decorator

**Decision**: `@friend(Class1, Class2)` for controlled access

**Rationale**:
- **Serialization**: JSON/DB mappers need private field access
- **Testing**: Test classes need internal verification
- **Explicit grant**: Must be declared by the class being accessed
- **Compile-time**: No runtime overhead

**Why not package-private?**
- Too coarse-grained (entire package is friends)
- Desi modules can be large
- Explicit is more maintainable

### 7. `__del__` Destructor

**Decision**: Support Python-style destructor for RAII

**Rationale**:
- **Resource safety**: File handles, sockets, locks need cleanup
- **Python familiarity**: `__del__` is known pattern
- **Deterministic**: Called immediately when freed (not GC'd)
- **Arena-compatible**: Works with arena scope exit

**Comparison**:
- Python: `__del__` (but non-deterministic due to GC)
- Rust: `Drop` trait (deterministic, same as Desi goal)
- C++: Destructor `~Class()` (deterministic)

---

### Design Principles Summary

| Principle | Implementation |
|-----------|----------------|
| **Python familiarity** | Similar syntax, dunders, decorators |
| **Explicit is better** | Manual memory, explicit pub/friend |
| **Gradual safety** | Choose your memory model |
| **Zero hidden costs** | No implicit outer refs, no hidden GC |
| **Systems-friendly** | C-compatible layout, FFI support |

---



## Current Implementation Status

| Feature | Status | Location |
|---------|--------|----------|
| Basic class definition | ✅ Complete | `parse/decl_class.go` |
| Fields (pub/private) | ✅ Complete | `check/expr_field.go` |
| Methods & self | ✅ Complete | `check/check_type.go` |
| Constructors (`__new__`) | ✅ Complete | `check/expr_call.go` |
| Single inheritance | ✅ Complete | `lower/class_lower.go` |
| Generic classes | ✅ Complete | `lower/class_monomorph.go` |
| Static methods | ✅ Complete | `check/expr_field.go` |
| Class methods | ✅ Complete | `check/expr_field.go` |
| Properties | ✅ Complete | `check/expr_field.go` |
| Abstract classes | ✅ Complete | `check/check_type.go` |
| Dunders (`__len__`, etc.) | ✅ Complete | `check/expr_call.go` |
| Class constants | ✅ Complete | `check/expr_field.go` |
| Static fields | ✅ Complete | `lower/lower.go` |
| Nested classes | 🚧 Partial | See [Nested Classes](#nested-classes) |
| `@friend` decorator | 📋 Planned | See [Friend Access](#friend-access-pattern) |
| `__del__` destructor | 📋 Planned | See [Memory Management](#memory-management) |

---

## Class Visibility Model

### Top-Level Classes

**Default: PUBLIC**

```desi
class MyClass:  # PUBLIC by default - can be imported
    pass
```

Equivalent to:
```desi
pub class MyClass:  # Explicit pub (redundant for top-level)
    pass
```

**Parser logic** (`parse/decl_class.go` line 312):
```go
decl := &ast.ClassDecl{
    Pub: explicitPub || !isNested,  // Top-level = public by default
}
```

### Nested Classes

**Default: PRIVATE**

```desi
class Outer:
    class Inner:      # PRIVATE - only accessible within Outer
        pass
    
    pub class Public: # PUBLIC - can be accessed via Outer.Public
        pass
```

### Visibility Rules Matrix

| Scenario | Outer Class | Nested (no pub) | Nested (pub) |
|----------|-------------|-----------------|--------------|
| Same file, any function | ✅ | ❌ | ✅ via `Outer.Nested` |
| Outer class methods | ✅ | ✅ | ✅ |
| Cross-file import | ✅ | ❌ | ✅ `from x import Outer.Nested` |

### AST Representation

```go
// ast/class_node.go
type ClassDecl struct {
    Pub        bool           // Visibility flag
    Name       Ident
    TypeParams []Ident
    Bases      []*TypeName
    Methods    []*FuncDecl
    Fields     []*FieldDecl
    Nested     []*ClassDecl   // Nested class declarations
    // ...
}
```

---

## Nested Classes

### Design Principles

1. **Static nesting only** - No implicit outer reference (unlike Java inner classes)
2. **Namespace organization** - Nested classes are for logical grouping
3. **Private by default** - Encapsulation is preserved
4. **Explicit outer reference** - If needed, pass outer explicitly

### Syntax Examples

```desi
class Database:
    class Config:  # Private nested class
        pub host: str
        pub port: int
    
    pub class Connection:  # Public nested class
        pub def __new__(self, config: Database.Config):
            pass
    
    config: Config  # Can reference private nested class within Outer
    
    pub def connect(self) -> Connection:
        return Database.Connection(self.config)
```

### Access Patterns

**Within parent methods (✅ ALLOWED)**:
```desi
class Outer:
    class Inner:
        pub val: int
    
    pub def create_inner(self) -> int:
        let i = Outer.Inner()  # ✅ Access via fully qualified name
        i.val = 42
        return i.val
```

**Cross-file with pub (✅ ALLOWED)**:
```desi
# file: models.desi
class User:
    pub class Profile:
        pub bio: str

# file: main.desi
from models import User
let p = User.Profile()  # ✅ Profile is pub
```

**Cross-file without pub (❌ ERROR)**:
```desi
# file: internal.desi
class Engine:
    class InternalState:  # Private
        pass

# file: main.desi
from internal import Engine
let s = Engine.InternalState()  # ❌ ERROR: InternalState is private
```

### Family Access Rule

**Inner classes can access Outer's private members**:

```desi
class Outer:
    _secret: int = 42
    
    class Inner:
        pub def peek(self, outer: Outer) -> int:
            return outer._secret  # ✅ ALLOWED - "family" access
    
def external_func(o: Outer) -> int:
    return o._secret  # ❌ ERROR - Not family
```

### Implementation TODO (expr_field.go)

```go
// In typFieldExpr, after checking static/class methods:
// Check for nested class access: Outer.Inner
if classType.Decl != nil {
    for _, nested := range classType.Decl.Nested {
        if nested.Name.Name == fieldName {
            // Check visibility
            if !nested.Pub && !isWithinClass(c, classType) {
                c.add(diagAt("DTE0010", x.Name.Span, 
                    "nested class '"+fieldName+"' is private"))
                return nil
            }
            // Look up the nested class type
            nestedSym := c.lookupNestedClass(classType, fieldName)
            if nestedSym != nil {
                c.info.Types[x] = nestedSym.Type
                return nestedSym.Type
            }
        }
    }
}
```

---

## Memory Management

### Current State

| Feature | Status | Details |
|---------|--------|---------|
| Heap allocation | ✅ | All class instances malloc'd |
| Manual free | ✅ | Collections have `.free()` |
| Arena allocation | ✅ | `using arena:` scope |
| Reference counting | 🚧 | Types exist, runtime partial |
| RAII/Destructors | 📋 | `__del__` not implemented |

### Memory Safety Levels (Proposed)

#### Level 1: Basic (Current)
```desi
class Point:
    pub x: int
    pub y: int

let p = Point()  # malloc'd, no automatic free
# Developer must manage lifetime
```

#### Level 2: Arena-Managed
```desi
using arena:
    let p = Point()  # Arena-allocated
    process(p)
# All objects freed when arena scope ends
```

#### Level 3: Reference Counted
```desi
let p = Rc(Point())  # Reference counted
let p2 = p.clone()   # Increment refcount
# When all Rc's go out of scope, Point is freed
```

#### Level 4: Unique Ownership (Future)
```desi
let p = Box(Point())  # Unique owner
let p2 = p           # MOVES p, p is now invalid
# p2 drop triggers free
```

### Destructor (`__del__`) Design

```desi
class FileHandle:
    _handle: int
    
    pub def __new__(self, path: str):
        self._handle = open_file(path)
    
    pub def __del__(self):  # Called on free
        close_file(self._handle)

# Usage with arena
using arena:
    let f = FileHandle("data.txt")
    process(f)
# __del__ called automatically here
```

### Memory Safety Rules for Nested Classes

1. **No implicit outer reference** - Nested doesn't hold Outer unless explicit
2. **Independent lifetimes** - Nested instance can outlive Outer
3. **Explicit dependency** - If Inner needs Outer, use `Rc` or arena

```desi
class Outer:
    class Inner:
        pub outer_ref: Outer  # Explicit reference
        
using arena:
    let o = Outer()
    let i = Outer.Inner()
    i.outer_ref = o
# Both freed together with arena
```

---

## Friend Access Pattern

### Design

The `@friend` decorator allows specific external classes to access private members.

```desi
@friend(JsonSerializer, TestHelper)
class User:
    _password_hash: str
    _email: str
    
    pub def __new__(self, email: str):
        self._email = email

class JsonSerializer:
    pub def serialize(self, user: User) -> str:
        # ✅ ALLOWED - JsonSerializer is a friend
        return f'{{"email": "{user._email}"}}'

class Hacker:
    pub def steal(self, user: User) -> str:
        return user._password_hash  # ❌ ERROR - Not a friend
```

### Friend Rules

| Rule | Behavior |
|------|----------|
| Explicit | Must list friend classes by name |
| Non-transitive | Friends of friends are NOT friends |
| Non-inheritable | Subclass of friend is NOT a friend |
| Non-mutual | A friends B ≠ B friends A |
| Class-level | Applies to entire class, not individual methods |

### Implementation (Future)

```go
// types/types.go - Add to Class struct
type Class struct {
    // ... existing fields
    Friends []string  // Friend class names
}

// check/expr_field.go - Access check
func (c *checker) checkPrivateAccess(field *types.Field, targetClass *types.Class) bool {
    if field.IsPub {
        return true
    }
    // Check if current class is a friend
    currentClass := c.getCurrentClass()
    if currentClass != nil {
        for _, friend := range targetClass.Friends {
            if currentClass.Name == friend {
                return true
            }
        }
    }
    // Check family access (nested)
    if c.isWithinClass(targetClass) {
        return true
    }
    return false
}
```

---

## Implementation Files

| File | Purpose |
|------|---------|
| `ast/class_node.go` | ClassDecl, FieldDecl, nested class AST |
| `parse/decl_class.go` | Class parsing, visibility flags |
| `check/check_type.go` | Class type building, inheritance |
| `check/expr_field.go` | Field/method access, static access |
| `check/expr_call.go` | Constructor calls, overload resolution |
| `lower/class_lower.go` | HIR generation, method lowering |
| `lower/class_monomorph.go` | Generic class monomorphization |
| `types/types.go` | Class type definition |
| `types/send_sync.go` | Thread safety traits |

---

## Phased Implementation Roadmap

### Phase 1: Nested Class Access (HIGH PRIORITY)
- [ ] Fix `Outer.Inner` syntax in type checker
- [ ] Add visibility check for nested classes
- [ ] Add family access rule
- [ ] Create test cases

**Files to modify**: `check/expr_field.go`, `check/scope.go`

### Phase 2: Cross-File Nested Import
- [ ] Parser: Handle `from x import Outer.Inner` syntax
- [ ] Resolver: Export nested classes with qualified name
- [ ] Update `exports.go` for nested class exports

**Files to modify**: `parse/stmt.go`, `resolve/exports.go`

### Phase 3: `__del__` Destructor
- [ ] Add `__del__` dunder recognition
- [ ] Lower destructor call in `hir.Free`
- [ ] Integrate with arena cleanup
- [ ] Test RAII patterns

**Files to modify**: `check/check_type.go`, `lower/lower_stmt.go`

### Phase 4: Reference Counting (`Rc<T>`)
- [ ] Complete `Rc` type implementation
- [ ] Add refcount increment/decrement in lowering
- [ ] Implement `clone()` method
- [ ] Add weak reference support

**Files to modify**: `types/rc.go`, `lower/lower_call.go`, `runtime/rc.c`

### Phase 5: Friend Access
- [ ] Parser: Handle `@friend(Class1, Class2)` decorator
- [ ] Type checker: Friend visibility rules
- [ ] Compile-time friend validation

**Files to modify**: `parse/decl_class.go`, `check/expr_field.go`

### Phase 6: Ownership & Move Semantics
- [ ] Track ownership in type checker
- [ ] Implement move detection
- [ ] Add use-after-move errors
- [ ] `Box<T>` unique ownership

**Files to modify**: `check/stmt.go`, `types/ownership.go`

---

## Cross-Language Comparison

| Feature | Python | Java | Rust | C++ | **Desi** |
|---------|--------|------|------|-----|----------|
| Top-level default | Public | Package | Private | Public | **Public** |
| Nested default | Public* | Package | Private | Private | **Private** |
| Implicit outer ref | Yes | Yes (inner) | No | No | **No** |
| Private access by nested | Yes | Yes | Yes | Yes | **Yes (family)** |
| Friend classes | No | No | No | Yes | **Yes (planned)** |
| Destructors | `__del__` | finalize | Drop | ~Class | **`__del__` (planned)** |
| GC | Yes | Yes | No | No | **No (manual/arena/Rc)** |

*Python has no true private, only name mangling convention

---

## Test Cases

### Nested Class Access
```desi
# examples/225_nested_class_access.desi
class Outer:
    class Inner:
        pub val: int
    
    pub def use_inner(self) -> int:
        let i = Outer.Inner()
        i.val = 42
        return i.val

def main() -> int:
    let o = Outer()
    print(o.use_inner())  # 42
    return 0
```

### Visibility Error
```desi
# Should produce DTE0010
class Outer:
    class PrivateInner:
        pass

def main() -> int:
    let x = Outer.PrivateInner()  # ERROR: private
    return 0
```

### Family Access
```desi
class Outer:
    _secret: int = 42
    
    class Inner:
        pub def access(self, o: Outer) -> int:
            return o._secret  # OK - family

def main() -> int:
    return 0
```

---

## References

- [Python Data Model](https://docs.python.org/3/reference/datamodel.html)
- [Rust Visibility](https://doc.rust-lang.org/reference/visibility-and-privacy.html)
- [Java Nested Classes](https://docs.oracle.com/javase/tutorial/java/javaOO/nested.html)
- [C++ Friend Classes](https://en.cppreference.com/w/cpp/language/friend)

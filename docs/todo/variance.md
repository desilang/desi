# TODO: Variance in Generics

**Status**: Proposed  
**Priority**: Low (Use invariant for now)  
**Related**: Generics, Type system safety

## Overview

Variance determines how generic types relate when their type parameters have subtype relationships.

## Quick Summary

**For Desi**: Use **invariant** (default) for all generics initially. This is the safest option and what most languages do.

## What is Variance?

Given types `Dog <: Animal` (Dog is a subtype of Animal):
- **Covariant (+)**: `Box<Dog> <: Box<Animal>` ✓
- **Contravariant (-)**: `Box<Animal> <: Box<Dog>` ✓  
- **Invariant**: `Box<Dog>` and `Box<Animal>` are unrelated ✗

## Examples with Pseudo Code

### Covariant (Read-Only Types)

```desi
# Covariant type parameter (hypothetical syntax)
trait Iterator<+T>:
    def next(self) -> Option<T>  # Only produces T, never consumes

# Safe because:
class Animal: pass
class Dog(Animal): pass

let dog_iter: Iterator<Dog> = ...
let animal_iter: Iterator<Animal> = dog_iter  # ✅ SAFE

# When we call animal_iter.next(), we expect Animal
# But we get Dog, which IS-A Animal, so it's fine!
```

**Rule**: Covariant when type appears only in **output positions** (return types).

### Contravariant (Write-Only Types)

```desi
# Contravariant type parameter (hypothetical syntax)
trait Consumer<-T>:
    def consume(self, item: T)  # Only consumes T, never produces

# Safe because:
let animal_consumer: Consumer<Animal> = ...
let dog_consumer: Consumer<Dog> = animal_consumer  # ✅ SAFE

# When we call dog_consumer.consume(dog), we pass Dog
# But animal_consumer.consume() accepts Animal
# Since Dog IS-A Animal, it's accepted!
```

**Rule**: Contravariant when type appears only in **input positions** (parameter types).

### Invariant (Read-Write Types) - DEFAULT

```desi
# Invariant type parameter (default, no annotation)
type Box<T>:
    value: T
    
    def get(self) -> T:      # Output position
        return self.value
    
    def set(self, val: T):   # Input position
        self.value = val

# NOT safe:
let dog_box: Box<Dog> = Box(Dog())
let animal_box: Box<Animal> = dog_box  # ❌ UNSAFE!

# Why? We could:
animal_box.set(Cat())  # Cat IS-A Animal, should be allowed
# But dog_box now contains a Cat! Type system broken!
```

**Rule**: Invariant when type appears in **both input and output positions**.

## Desi's Approach

### Current: All Invariant

```desi
struct Box<T>:
    value: T

# Box<Dog> and Box<Animal> are completely unrelated
# Even if Dog <: Animal
```

This is **safe but restrictive**.

### Future: Explicit Variance Annotations

```desi
# Covariant (read-only)
trait Iterator<+T>:
    def next(self) -> Option<T>

# Contravariant (write-only)
trait Consumer<-T>:
    def consume(self, item: T)

# Invariant (default, read-write)
struct Box<T>:
    value: T
```

## Real-World Examples

### Example 1: Collections

```desi
# List is covariant in most languages for convenience
type List<+T>:  # Covariant
    # ...

class Animal: pass
class Dog(Animal): pass

let dogs: List<Dog> = [Dog()]
let animals: List<Animal> = dogs  # ✅ Would be allowed with variance

# Safe IF list is immutable or read-only
```

**Problem**: If list is mutable, this is unsafe!

```desi
animals.append(Cat())  # ❌ Now dogs list contains a Cat!
```

**Solution**: Separate mutable and immutable types:
```desi
type ReadOnlyList<+T>:  # Covariant, safe
    def get(self, i: int) -> T

type MutableList<T>:     # Invariant, safe
    def get(self, i: int) -> T
    def set(self, i: int, val: T)
```

### Example 2: Function Types

```desi
# Function types have special variance:
# fn(A) -> B  is equivalent to Function<-A, +B>
#   ^contravariant in input
#              ^covariant in output

# Example:
type Handler = fn(Animal) -> Dog

# We can substitute with:
let h1: fn(Dog) -> Animal = ...  # ❌ UNSAFE
let h2: fn(Animal) -> Dog = ...  # ✅ Exact match
let h3: fn(Cat) -> Dog = ...     # ❌ UNSAFE (can't handle all Animals)
let h4: fn(Animal) -> GermanShepherd = ...  # ✅ SAFE

# Why is h4 safe?
# Handler expects: fn(Animal) -> Dog
# We provide:     fn(Animal) -> GermanShepherd
# When we call, we pass Animal, it accepts Animal ✓
# When we get result, we expect Dog, we get GermanShepherd (IS-A Dog) ✓
```

## Recommendation for Desi

### Phase 1 (Current): Invariant Everything
✅ Simple  
✅ Safe  
✅ Easy to reason about  
❌ Less flexible  

### Phase 2 (Future): Add Variance When Needed

Only add variance if users complain about specific patterns being rejected.

Likely candidates for covariance:
- `Iterator<+T>`
- `Future<+T>`
- `Option<+T>` (already immutable)
- `Result<+T, +E>`

## Implementation Notes

Variance checking happens in the type checker:

```go
// In compiler/internal/check/check.go

fn check_assignability(target: Type, source: Type) -> bool {
    match (target, source) {
        (Generic { base: b1, args: a1 }, Generic { base: b2, args: a2 }) => {
            if b1 != b2 { return false; }
            
            // Check each type argument according to variance
            for (arg1, arg2, variance) in zip(a1, a2, b1.variances) {
                match variance {
                    Covariant => if !is_subtype(arg2, arg1) { return false; }
                    Contravariant => if !is_subtype(arg1, arg2) { return false; }
                    Invariant => if arg1 != arg2 { return false; }
                }
            }
            true
        }
        // ... other cases
    }
}
```

## References

- Scala: Declaration-site variance (`+T`, `-T`)
- Java: Use-site variance (`? extends T`, `? super T`)
- C#: Declaration-site variance (`out T`, `in T`)
- Rust: Variance is inferred by compiler
- **Desi**: Start with invariant, add variance if needed

## Task Breakdown

- [ ] Document current invariant behavior
- [ ] Wait for user feedback on limitations
- [ ] If needed, design variance syntax
- [ ] Implement variance annotations in AST
- [ ] Add variance checking to type checker
- [ ] Write comprehensive tests
- [ ] Document variance rules for users

**Current status**: Not needed. Use invariant (default).

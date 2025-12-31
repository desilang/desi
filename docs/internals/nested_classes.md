# Nested Classes Implementation

Desi supports nested classes (classes defined within other classes), including generic nested classes.

## Features

- **Nested Definition:** `class Outer { class Inner { ... } }`
- **Generics:** `class Outer { class Box[T] { ... } }`
- **Cross-File Access:** Access via `Outer.Inner`.

## Implementation Details

### Symbol Mangling
To support linking and runtime uniqueness, nested classes are mangled with their parent's name.
- `Outer.Inner` -> `Outer_Inner`
- `Container.Box[int]` -> `Container_Box_int` (Monomorphization)

### Initializer Lowering
The constructor lowering logic (`lower_call.go`) was updated to handle nested types.
- It detects if a call `Container.Box(42)` refers to a nested class.
- It resolves the fully qualified name for the backend constructor call.
- For generics, it performs type inference based on arguments (e.g., `42` -> `int`).

### Checker (`check/decl.go`)
- Registers nested classes in the parent's `Classes` map.
- Ensures correct scope visibility for type parameters. A nested class within a generic outer class can access outer type parameters (Not fully implemented/tested yet, but independent generics `Box[T]` work).

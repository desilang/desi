# Enum Implementation Edge Cases & Requirements

## 1. Enum Types
### Simple Enums (No Payload)
```python
enum Status {
    Pending,
    Running,
    Completed,
    Failed
}
```
- **Edge Case**: Zero variants (`enum Void {}`).
- **Edge Case**: Single variant.
- **Requirement**: Should be comparable (`Status.Pending == Status.Pending`).
- **Requirement**: Should print as string ("Status.Pending").

### Data Enums (Tagged Unions)
```python
enum Shape {
    Circle(radius: float),
    Rectangle(width: float, height: float),
    Point  # Mixed with no-payload variant
}
```
- **Edge Case**: Payload types (primitives vs structs vs other enums).
- **Edge Case**: Multiple fields in a variant (`Rectangle`).
- **Edge Case**: Mixed variants (some with data, some without).

### Recursive Enums
```python
enum LinkedList {
    Node(value: int, next: LinkedList),
    End
}
```
- **Edge Case**: Infinite size if not handled via pointers. The compiler must ensure recursive types are boxed (pointers).

## 2. Usage & Semantics
### Instantiation
- Syntax: `Shape.Circle(radius=5.0)` or `Shape.Point`.
- **Edge Case**: Missing arguments for data variant.
- **Edge Case**: Extra arguments for empty variant.

### Matching (Match Expressions)
```python
match shape {
    case Shape.Circle(r): print(r)
    case Shape.Rectangle(w, h): print(w * h)
    case Shape.Point: print("Point")
}
```
- **Edge Case**: Non-exhaustive match (missing variants).
- **Edge Case**: Unused variables in match arms.
- **Edge Case**: Nested matching (matching an enum inside an enum).
- **Edge Case**: Wildcard match (`case _`).

### Equality
- `Shape.Circle(5.0) == Shape.Circle(5.0)` should be true.
- `Shape.Circle(5.0) == Shape.Circle(6.0)` should be false.
- `Shape.Circle(5.0) == Shape.Point` should be false.

## 3. Memory Layout (Implementation Details)
- **Tag**: Need a discriminator (integer) to distinguish variants.
- **Payload**: Space for the largest variant's data.
- **Strategy**:
  - `struct { tag: i32, payload: ptr }` (Simpler, payload is malloc'd separately)
  - OR `struct { tag: i32, data: [N x i8] }` (Inline union, complex to lower)
  - **Decision**: For MVP, use `struct { tag: i32, payload: ptr }`.
    - `tag`: 0 for first variant, 1 for second, etc.
    - `payload`: Pointer to a struct specific to that variant (or null if no data).

## 4. Type Checking
- Need `types.Enum` and `types.Variant`.
- Namespace resolution: `EnumName.VariantName`.
- Type compatibility: `Shape.Circle` is of type `Shape`.

## 5. Generic Enums (Future/Advanced)
- `enum Option[T] { Some(T), None }`
- **Note**: Postpone for now until basic enums work.

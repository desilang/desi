# Marshal Module

The `marshal` module provides functions for custom binary serialization and deserialization of Desi values. This is useful for writing data to files, transmitting it over sockets, or caching data.

Compared to JSON, the `marshal` module produces smaller binary payloads and is significantly faster, as it avoids text parsing and formatting overhead.

## API Reference

### `dumps`

```desi
pub def dumps<T>(val: T) -> bytes
```

Serializes a value of type `T` into a binary `bytes` object.

Supported types for `T` include:
*   Primitives (`int`, `float`, `bool`, `str`)
*   Lists (`list[T]`)
*   Dictionaries (`dict[K, V]`)
*   Tuples (`tuple[A, B, ...]`)

### `loads`

```desi
pub def loads<T>(data: bytes) -> T
```

Deserializes a binary `bytes` object back into a value of type `T`.

> [!WARNING]
> You must specify the generic type argument explicitly (e.g. `marshal.loads::<list[int]>(data)`) so the compiler knows the expected return type and structure of the deserialized data.

## Example Usage

```desi
import marshal

def main() -> int:
    # 1. Prepare data
    let original = {"name": "Desi", "version": 1}
    
    # 2. Serialize to bytes
    let serialized = marshal.dumps(original)
    print(f"Serialized size: {len(serialized)} bytes")
    
    # 3. Deserialize back
    let restored = marshal.loads::<dict[str, int]>(serialized)
    
    # 4. Verify results
    assert restored["name"] == "Desi"
    assert restored["version"] == 1
    print("Marshal test passed!")
    
    return 0
```

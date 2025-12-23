# Custom Class Iteration

You can create iterable custom classes by implementing `__getitem__` and `__len__`.

## Required Methods

| Method | Signature | Purpose |
|--------|-----------|---------|
| `__getitem__` | `(self, index: int) -> T` | Get element at index |
| `__len__` | `(self) -> int` | Return number of elements |

## Example

```desi
class MyList:
    pub data: list[int]
    
    pub def __new__(self, values: list[int]):
        self.data = values
    
    pub def __getitem__(self, index: int) -> int:
        return self.data[index]
    
    pub def __len__(self) -> int:
        return len(self.data)

def main() -> int:
    let arr = MyList([10, 20, 30, 40, 50])
    
    for x in arr:
        print(x)  # Prints: 10 20 30 40 50
    
    return 0
```

## How It Works

When you write `for x in custom_obj:`, the compiler:

1. Calls `obj.__len__()` to get the count
2. Loops from 0 to count-1
3. Calls `obj.__getitem__(index)` for each element

## Negative Indexing

Since `__getitem__` is called directly, negative indexing works if your implementation supports it:

```desi
pub def __getitem__(self, index: int) -> int:
    return self.data[index]  # data[-1] will work!
```

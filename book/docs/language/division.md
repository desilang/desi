# Division by Zero

Division by zero is a runtime error in Desi. Both integer and float division by zero cause the program to panic.

## Behavior

```desi
let x = 10 / 0      # Panics!
let y = 10.0 / 0.0  # Panics!
let z = 10 % 0      # Panics! (modulo also checks)
```

When dividing by zero:
1. Error message printed to stderr: `panic: integer division by zero`
2. Program exits with code 1

## Why Panic?

Unlike some languages that return `inf` or `NaN` for float division by zero, Desi treats all division by zero as an error because:

1. **Safety**: Silent corruption (returning 0 or `inf`) can cause bugs that are hard to track down
2. **Consistency**: Same behavior for int and float makes the language predictable
3. **Explicitness**: Errors should be visible, not hidden

## Safe Division

Check before dividing:
```desi
def safe_divide(a: int, b: int) -> int:
    if b == 0:
        print("Cannot divide by zero", file=sys.stderr)
        return 0
    return a / b
```

## Future: Result Types

In future versions, Desi may offer a `checked_div` function:
```desi
# Future syntax
let result = checked_div(10, 0)  # Returns Result[int, DivError]
match result:
    case Ok(v): print(v)
    case Err(e): print(e)
```

## See Also

- [Error Handling](./error-handling.md) - For Result types

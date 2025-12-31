# Pipeline Operator

The pipeline operator (`|>`) allows you to chain function and method calls in a readable, left-to-right flow. It passes the result of the expression on the left as the **first argument** to the function or method on the right.

## Syntax

```desilang
value |> function(other_args)
```

Is equivalent to:

```desilang
function(value, other_args)
```

## Examples

### With Functions

```desilang
def add(a: int, b: int) -> int:
    return a + b

def square(x: int) -> int:
    return x * x

let result = 5 |> add(3) |> square()
# Equivalent to: square(add(5, 3))
# (5 + 3)^2 = 64
```

### With Methods

The pipeline operator also works with class methods. The pipe value is injected as the first explicit argument.

```desilang
class Calculator:
    pub def sub(self, a: int, b: int) -> int:
        return a - b

let calc = Calculator()

# 10 is passed as 'a', 5 is passed as 'b'
let res = 10 |> calc.sub(5)
# Equivalent to: calc.sub(10, 5)
# Result: 5
```

### Chaining

Pipelines are excellent for deep chaining of operations, improving readability by avoiding deeply nested parentheses.

```desilang
# Without pipeline
let final = step3(step2(step1(data)))

# With pipeline
let final = data |> step1() |> step2() |> step3()
```

# Async & Await in Desi

Desi supports asynchronous programming with `async def` and `await`. Async functions run on background threads and return `Future` handles that you can await later.

## Quick Start

```desi
async def add1(x: int) -> int:
    return x + 1

def main() -> int:
    let fut = add1(41)    # Spawns on background thread, returns immediately
    let n = await fut      # Blocks until result is ready
    print(str(n))          # → 42
    return 0
```

## Defining Async Functions

Add `async` before `def` to make a function asynchronous:

```desi
# Async function returning int
async def compute(x: int) -> int:
    return x * x

# Async function returning str
async def greet(name: str) -> str:
    return "Hello " + name

# Async function with multiple parameters
async def add(a: int, b: int) -> int:
    return a + b
```

When you call an async function, it immediately returns a `Future` — the actual work runs on a background thread.

## Awaiting Results

Use `await` to get the result from a `Future`:

```desi
let fut = compute(7)   # Returns Future immediately
# ... do other work here ...
let result = await fut  # Blocks until compute() finishes
print(str(result))      # → 49
```

> ⚠️ **Important:** `await` blocks the current thread until the async function completes. The async function itself runs on a separate thread.

## Parallel Execution

Launch multiple async functions and await them all:

```desi
async def double(x: int) -> int:
    return x * 2

def main() -> int:
    # Both run concurrently on separate threads
    let f1 = double(10)
    let f2 = double(20)
    
    # Await both results
    let a = await f1    # → 20
    let b = await f2    # → 40
    
    print(str(a + b))   # → 60
    return 0
```

Both `double(10)` and `double(20)` start running immediately when called. The `await` calls collect the results.

## Supported Return Types

Async functions work with all Desi types:

```desi
# Primitives
async def get_number() -> int:
    return 42

async def get_flag() -> bool:
    return true

async def get_name() -> str:
    return "Desi"

# Collections and user-defined types work too
# (structs, classes, lists, dicts — anything that's a pointer type)
```

## Complete Example

```desi
async def fetch_greeting(name: str) -> str:
    return "Hello, " + name + "!"

async def fetch_count() -> int:
    return 42

def main() -> int:
    # Launch both async operations
    let greeting_fut = fetch_greeting("Desi")
    let count_fut = fetch_count()
    
    # Await results
    let greeting = await greeting_fut
    let count = await count_fut
    
    print(greeting)           # → Hello, Desi!
    print(str(count))         # → 42
    return 0
```

## How It Works

Under the hood, `async def` creates two functions:

1. **Wrapper**: Creates a `Future`, spawns the body on a background thread, returns the `Future`
2. **Body**: Runs the actual function code on the background thread

```
main thread                    background thread
───────────                    ─────────────────
fut = fetch_greeting("Desi")
  └─ creates Future
  └─ spawns body thread ──────► body("Desi") runs
  └─ returns Future              │
                                 │ computes result
...do other work...             │
                                 ▼
greeting = await fut ◄────────── future completes
```

## Borrowing Rules

Desi enforces that `inout` (mutable borrow) parameters cannot be held across `await` points:

```desi
# ❌ COMPILE ERROR: inout borrow across await
async def bad(inout x: int) -> int:
    let fut = some_async_call()
    await fut           # Error! inout 'x' held across await
    return x
```

This prevents data races — if the original value could change while the async function is suspended, the borrow could become invalid.

## Current Limitations

- **No async closures/lambdas** yet — only named functions can be `async`
- **No async methods** yet — class/struct methods cannot be `async`
- **No cancellation** — once spawned, an async function runs to completion
- **Thread per call** — each async call spawns a new pthread (no thread pool yet)
- **Must await** — if a `Future` is never awaited, the thread still runs but resources may leak

## See Also

- [TaskGroup](./taskgroup.md) — Structured concurrency for multiple tasks
- [Mutex](./mutex.md) — Protecting shared data between threads
- [Channel](./channel.md) — Message passing between threads

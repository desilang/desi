
# 09 · Async Basics

Async functions use `async def`; `await` inside; `task.block_on` to run.

```desi
import task

async def foo() -> int:
  await task.sleep_ms(50)
  42

def main() -> int:
  let fut = foo()
  let x = task.block_on(fut)
  print("result", x)
  0
```


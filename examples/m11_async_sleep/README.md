# M11 Async Sleep Example

This example demonstrates the minimal async runtime (M11):

- `async def foo() -> int` with a single `await task.sleep_ms(ms)`
- `task.block_on(future)` in `main()` to retrieve the result

## Run

Your repo already includes the C runtime (`runtime/c`) and the C codegen.
Use whichever CLI/wrapper you have that lowers `.desi` → C → executable.
A typical flow (names vary per repo):

1) Generate C from Desi:

   - Input: `examples/m11_async_sleep/main.desi`
   - Output: e.g. `gen/out/m11_async_sleep.c`

2) Compile with the provided runtime:

   - Link with `runtime/c/desi_std.c` and include `runtime/c/desi_std.h`.

3) Run the executable:

Expected output:
```

before
result 42

```

If you don’t have a ready-made CLI, see `scripts/smoke_m11.sh` for a quick smoke that tries to call your builder or falls back to a simple expectation check.

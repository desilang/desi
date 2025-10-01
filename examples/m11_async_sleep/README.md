# M11 Async Sleep Example

This example demonstrates the minimal async runtime (M11):

- `async def foo() -> int` with a single `await task.sleep_ms(ms)`
- `task.block_on(future)` in `main()` to retrieve the result

## Quick run (your current flow)

```sh
go run ./compiler/cmd/desic build examples/m11_async_sleep/main.desi
./gen/out/main
```

Expected output:

```
before
result 42
```

## Smoke script

You can also run the smoke script; it now auto-detects the builder:

```sh
scripts/smoke_m11.sh
```

It will:

* build with `go run ./compiler/cmd/desic build examples/m11_async_sleep/main.desi`
* find the executable (usually `./gen/out/main`)
* run it and verify output

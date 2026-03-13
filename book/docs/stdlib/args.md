# args — CLI Argument Parsing

Simple, structured argument/flag parsing for command-line programs.

## Import

```desi
import args
```

## API Reference

| Function | Description |
|---|---|
| `args.count() -> int` | Argument count (excluding program name) |
| `args.program() -> str` | Program name (argv[0]) |
| `args.get_arg(i) -> str` | Get arg at index (0-based) |
| `args.argv() -> list[str]` | All arguments as list |
| `args.positional() -> list[str]` | Non-flag arguments only |
| `args.has_flag(name) -> int` | 1 if flag exists, 0 otherwise |
| `args.get_flag(name, default) -> str` | Flag value (or default) |
| `args.get_flag_int(name, default) -> int` | Flag as integer |
| `args.is_flag_set(name) -> int` | 1 if boolean flag set |

## Flag Formats

Both `--name value` and `--name=value` are supported:

```bash
./my_app --name Alice --port=3000 --verbose
```

## Examples

### Basic CLI App

```desi
import args

def main() -> int:
    let name = args.get_flag("--name", "World")
    let port = args.get_flag_int("--port", 8080)
    let verbose = args.has_flag("--verbose")

    print(f"Hello, {name}!")
    print(f"Listening on port {str(port)}")
    if verbose == 1:
        print("Verbose mode enabled")
    0
```

### Positional Args

```desi
import args

def main() -> int:
    let files = args.positional()
    for file in files:
        print(f"Processing: {file}")
    0
```

> **Note:** When using `desic run`, CLI args are not forwarded.
> Use `desic build` and run the compiled binary to test with real args.

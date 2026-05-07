# cli — Command-Line Interface Builder

Define, parse, and validate command-line flags and arguments.

## Import

```desi
import cli
```

## API Reference

### Basic Arguments

| Function | Description |
|---|---|
| `cli.count() -> int` | Number of arguments (excluding program name) |
| `cli.program() -> str` | Program name (argv[0]) |
| `cli.get(index) -> str` | Get argument at index (0-based) |

### Flags

| Function | Description |
|---|---|
| `cli.has_flag(name) -> bool` | Check if a flag was passed |
| `cli.get_flag(name, default) -> str` | Get flag value as string |
| `cli.get_flag_int(name, default) -> int` | Get flag value as integer |
| `cli.get_flag_bool(name) -> bool` | Check if boolean flag is set |

### Flag Definitions & Validation

| Function | Description |
|---|---|
| `cli.set_description(desc) -> int` | Set program description |
| `cli.define(long, short, desc, default) -> int` | Define a flag with default |
| `cli.define_bool(long, short, desc) -> int` | Define a boolean flag |
| `cli.require(flag_name) -> int` | Mark a flag as required |
| `cli.print_help() -> int` | Print auto-generated help |
| `cli.check_help() -> bool` | True if `--help` was passed |
| `cli.validate() -> str` | Validate flags, return error or empty string |

### Subcommands

| Function | Description |
|---|---|
| `cli.subcommand() -> str` | Get the subcommand name (first positional arg) |

## Flag Formats

Both `--name value` and `--name=value` are supported:

```bash
./myapp --host localhost --port=8080 --verbose
```

## Examples

### Simple Flag Parsing

```desi
import cli

def main() -> int:
    let name = cli.get_flag("--name", "World")
    let port = cli.get_flag_int("--port", 8080)
    let verbose = cli.get_flag_bool("--verbose")

    print(f"Hello, {name}!")
    print(f"Port: {str(port)}")
    if verbose:
        print("Verbose mode enabled")
    0
```

### Full CLI App with Validation

```desi
import cli

def main() -> int:
    cli.set_description("A sample Desi application")
    cli.define("--host", "-h", "Server hostname", "localhost")
    cli.define("--port", "-p", "Server port", "8080")
    cli.define_bool("--verbose", "-v", "Enable verbose output")
    cli.require("--host")

    # Auto-handle --help
    if cli.check_help():
        cli.print_help()
        return 0

    # Validate required flags
    let err = cli.validate()
    if err != "":
        print(f"Error: {err}")
        cli.print_help()
        return 1

    let host = cli.get_flag("--host", "localhost")
    let port = cli.get_flag_int("--port", 8080)
    print(f"Starting server on {host}:{str(port)}")
    0
```

### Subcommands

```desi
import cli

def main() -> int:
    let cmd = cli.subcommand()

    if cmd == "serve":
        print("Starting server...")
    elif cmd == "build":
        print("Building project...")
    else:
        print(f"Unknown command: {cmd}")
    0
```

# toml — TOML Config File Parsing

Parse TOML configuration files with section support and typed getters.

## Import

```desi
import toml
```

## API Reference

| Function | Description |
|---|---|
| `toml.parse(input) -> int` | Parse TOML string, returns entry count |
| `toml.parse_file(path) -> int` | Parse TOML file |
| `toml.get(key) -> str` | Get value by dotted key |
| `toml.get_default(key, def) -> str` | Get with default |
| `toml.get_int(key, def) -> int` | Get as integer |
| `toml.get_bool(key, def) -> int` | Get as bool (true/false/yes/no) |
| `toml.has_key(key) -> int` | Check if key exists |
| `toml.entry_count() -> int` | Number of parsed entries |

## Dotted Keys

Section headers create dotted keys:

```toml
[server]
port = 8080
host = "localhost"
```

Access as `toml.get("server.port")`.

## Examples

### Inline Config

```desi
import toml

toml.parse("[server]\nport = 8080\nhost = \"localhost\"")
let port = toml.get_int("server.port", 3000)
let host = toml.get("server.host")
print(f"Server: {host}:{str(port)}")
```

### File Config

```desi
import toml

toml.parse_file("config.toml")
let db = toml.get("database.url")
let debug = toml.get_bool("app.debug", 0)
```

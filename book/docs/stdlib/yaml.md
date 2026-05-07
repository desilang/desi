# yaml — YAML Parser

Parse and query YAML documents.

## Import

```desi
import yaml
```

## API Reference

| Function | Description |
|---|---|
| `yaml.parse(content) -> cptr` | Parse YAML from a string |
| `yaml.parse_file(path) -> cptr` | Parse YAML from a file |
| `yaml.get(h, path) -> str` | Get a string value by dot-path |
| `yaml.get_int(h, path) -> int` | Get an integer value |
| `yaml.get_bool(h, path) -> bool` | Get a boolean value |
| `yaml.has(h, path) -> bool` | Check if a path exists |
| `yaml.keys(h, path) -> str` | Comma-separated keys at path |
| `yaml.list_len(h, path) -> int` | Length of a list at path |
| `yaml.dump(h) -> str` | Serialize back to YAML string |
| `yaml.free(h) -> int` | Free the parsed YAML handle |

## Path Syntax

Use dot notation to access nested values:

- `"name"` — top-level key
- `"database.host"` — nested key
- `"servers.0.name"` — list index

## Examples

### Parse and Query

```desi
import yaml

def main() -> int:
    let content = "name: MyApp\nversion: 2\ndebug: true\ndatabase:\n  host: localhost\n  port: 5432"
    let doc = yaml.parse(content)

    print(yaml.get(doc, "name"))           # MyApp
    print(yaml.get_int(doc, "version"))    # 2
    print(yaml.get_bool(doc, "debug"))     # true
    print(yaml.get(doc, "database.host"))  # localhost

    print(yaml.has(doc, "database.port"))  # true
    print(yaml.has(doc, "missing"))        # false

    yaml.free(doc)
    0
```

### Parse from File

```desi
import yaml

def main() -> int:
    let doc = yaml.parse_file("config.yaml")

    let host = yaml.get(doc, "server.host")
    let port = yaml.get_int(doc, "server.port")
    print(f"Server: {host}:{str(port)}")

    yaml.free(doc)
    0
```

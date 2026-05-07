# ini — INI Configuration Files

Parse and query INI-style configuration files.

## Import

```desi
import ini
```

## API Reference

| Function | Description |
|---|---|
| `ini.parse(content) -> cptr` | Parse INI from a string |
| `ini.parse_file(path) -> cptr` | Parse INI from a file |
| `ini.get(h, section, key) -> str` | Get a value |
| `ini.get_default(h, section, key, default) -> str` | Get a value with fallback |
| `ini.has_section(h, section) -> bool` | Check if section exists |
| `ini.has_key(h, section, key) -> bool` | Check if key exists in section |
| `ini.sections(h) -> str` | Comma-separated list of sections |
| `ini.keys(h, section) -> str` | Comma-separated list of keys in section |
| `ini.free(h) -> int` | Free the parsed INI handle |

## INI Format

```ini
[database]
host = localhost
port = 5432
name = mydb

[server]
port = 8080
debug = true
```

## Examples

### Parse from File

```desi
import ini

def main() -> int:
    let cfg = ini.parse_file("config.ini")

    let host = ini.get(cfg, "database", "host")
    let port = ini.get(cfg, "database", "port")
    print(f"DB: {host}:{port}")

    if ini.has_section(cfg, "server"):
        let debug = ini.get_default(cfg, "server", "debug", "false")
        print(f"Debug mode: {debug}")

    ini.free(cfg)
    0
```

### Parse from String

```desi
import ini

def main() -> int:
    let content = "[app]\nname = MyApp\nversion = 1.0"
    let cfg = ini.parse(content)

    print(ini.get(cfg, "app", "name"))     # MyApp
    print(ini.get(cfg, "app", "version"))  # 1.0
    print(ini.sections(cfg))               # app

    ini.free(cfg)
    0
```

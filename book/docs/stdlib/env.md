# Env Module

The `env` module loads `.env` files into the process environment. Inspired by Node.js `dotenv`. **No other language ships this built-in.**

## Import

```desi
import env
```

## API Reference

| Function | Returns | Description |
|----------|---------|-------------|
| `load(path)` | `bool` | Load .env file from specific path |
| `load_default()` | `bool` | Load `.env` from current working directory |
| `get(key, default)` | `str` | Get env var with fallback default |
| `require(key)` | `str` | Get env var, **exit** if not set |
| `is_set(key)` | `bool` | Check if env var exists |

## .env File Format

```ini
# Comments start with #
DATABASE_URL=postgres://localhost/mydb
API_KEY="my-secret-key"
PORT=8080
DEBUG=true
```

- Lines starting with `#` are comments
- Surrounding quotes (`"` or `'`) are automatically stripped
- Empty lines are skipped
- Existing env vars are **not overwritten**

## Usage Example

```desi
import env

def main() -> int:
    env.load_default()

    let port = env.get("PORT", "8080")
    let db = env.get("DATABASE_URL", "sqlite://default.db")
    print(f"Starting on port {port}")
    print(f"Database: {db}")

    # Require critical config
    if env.is_set("API_KEY"):
        let key = env.require("API_KEY")
        print("API key loaded")
    0
```

## See Also

- [OS Module](os.md) — `os.getenv()` and `os.setenv()`

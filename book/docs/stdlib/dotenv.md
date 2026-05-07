# dotenv — Environment File Loading

Load `.env` files into the process environment.

## Import

```desi
import dotenv
```

## API Reference

| Function | Description |
|---|---|
| `dotenv.load(path = ".env") -> int` | Load a `.env` file (ignores missing files) |
| `dotenv.load_or_fail(path = ".env") -> int` | Load a `.env` file (fails if missing) |
| `dotenv.parse(content) -> str` | Parse `.env` content string |
| `dotenv.get(key) -> str` | Get an environment variable |

## `.env` File Format

```
DATABASE_URL=postgres://localhost:5432/mydb
API_KEY=sk-1234567890
DEBUG=true
```

## Examples

### Load and Use

```desi
import dotenv

def main() -> int:
    dotenv.load()  # loads .env from current directory
    let db = dotenv.get("DATABASE_URL")
    let key = dotenv.get("API_KEY")
    print(f"Database: {db}")
    print(f"API Key: {key}")
    0
```

### Custom Path

```desi
import dotenv

def main() -> int:
    dotenv.load(".env.production")
    let host = dotenv.get("HOST")
    print(host)
    0
```

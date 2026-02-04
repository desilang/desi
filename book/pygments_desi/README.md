# Desi Pygments Lexer

Syntax highlighting for Desi code in MkDocs and other Pygments-enabled tools.

## Installation

```bash
cd book
pip install -e .
```

## Usage in MkDocs

After installation, use `desi` in code blocks:

```markdown
```desi
def main() -> int:
    print("Hello, world!")
    0
```                  
```

## Features

- Keywords: `def`, `let`, `if`, `match`, `for`, `class`, etc.
- Types: `int`, `str`, `list`, `Option`, `Result`, etc.
- Decorators: `@extern`, `@ffi_struct`, etc.
- Operators: `->`, `|>`, `::`, `:=`
- Strings: Regular, f-strings, docstrings

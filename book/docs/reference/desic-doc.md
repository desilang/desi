# desic doc

Generate documentation from Desi source files.

## Quick Start

```bash
# Generate docs for a file
desic doc mymodule.desi

# Include private declarations
desic doc --all mymodule.desi
```

## Output

The command prints markdown to stdout. Redirect to save:

```bash
desic doc lib/sync/mutex.desi > docs/mutex.md
```

## What's Documented

| Declaration | Info Shown |
|-------------|------------|
| Functions | Signature, params, return type |
| Classes | Fields, methods, constants, inheritance |
| Structs | Fields |
| Enums | Variants with payloads |

Each item includes:
- Line number in source
- Docstring (if triple-quoted `"""..."""`)
- Visibility (`*(private)*` with `--all`)
- Generics (`<T: Bound>`)

## Example

```desi
pub class Stack<T>:
    """A simple generic stack."""
    pub mut items: list<T>
    
    pub def push(self, item: T) -> none:
        """Add item to top."""
        pass
```

Output:
```markdown
### `Stack<T>`
*Line 1*

A simple generic stack.

**Fields:**
- `items: list<T>` *(mut)*

**Methods:**
- `push(self, item: T) -> none` - Add item to top.
```

## Flags

| Flag | Description |
|------|-------------|
| `--all`, `-a` | Include private declarations |

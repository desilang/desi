# desic doc - Documentation Generator

The `desic doc` command generates markdown documentation from Desi source files.

## Usage

```bash
# Show only public declarations
desic doc <file.desi>

# Show all declarations (including private)
desic doc --all <file.desi>
desic doc -a <file.desi>
```

## Output Format

The command generates markdown with:

### Module Header
```markdown
# Module: filename.desi

Module-level docstring appears here if present.
```

A module docstring is the first triple-quoted string at the top of the file:
```desi
"""
This module provides utility functions.
"""

pub def helper() -> int:
    pass
```

### Functions
```markdown
### `function_name<T>(param: Type) -> ReturnType` *(private, async)*
*Line 42*

- `@decorator`

Docstring content here.
```

### Classes
```markdown
### `ClassName<T: Bound>` *(private)*
*Line 10*

- `@decorator`

**Extends:** BaseClass

Docstring content.

**Constants:**
- `PI: float`
- `SECRET: int` *(private)*

**Static Fields:**
- `count: int` *(mut)* *(private)*

**Fields:**
- `value: T` *(mut)*

**Methods:**
- `method_name(self: any) -> Type` *(async)* *(private)* - docstring summary
```

### Structs
```markdown
### `StructName` *(private)*
*Line 15*

**Fields:**
- `field: Type`
```

### Enums
```markdown
### `EnumName<T>` *(private)*
*Line 20*

**Variants:**
- `Variant1(Type)`
- `Variant2`
```

## Features

| Feature | Description |
|---------|-------------|
| Type parameters | `<T: Bound>` generic bounds |
| Line numbers | `*Line N*` source location |
| Visibility | `*(private)*` markers |
| Async/decorators | `*(async)*`, `@extern` |
| Inheritance | `**Extends:** Base` |
| Constants | Class-level `const` |
| Static fields | `pub mut static: Type` |
| Field mutability | `*(mut)*` markers |

## Implementation

Located in: `compiler/cmd/desic/main.go`

Key functions:
- `runDoc()` - CLI handler
- `generateDoc()` - Module-level doc generation
- `renderFunc()`, `renderClass()`, `renderStruct()`, `renderEnum()` - Declaration renderers
- `formatFuncSigFull()` - Full signature with type params
- `formatTypeName()` - Generic type formatting

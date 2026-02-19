# Color Module

The `color` module provides terminal text styling with ANSI escape codes. Includes foreground colors, backgrounds, text styles, and 24-bit true color. **No other language ships this built-in.**

## Import

```desi
import color
```

## API Reference

### Foreground Colors

| Function | Example Output |
|----------|---------------|
| `red(s)` | Red text |
| `green(s)` | Green text |
| `yellow(s)` | Yellow text |
| `blue(s)` | Blue text |
| `magenta(s)` | Magenta text |
| `cyan(s)` | Cyan text |
| `white(s)` | White text |
| `gray(s)` | Gray text |

### Text Styles

| Function | Effect |
|----------|--------|
| `bold(s)` | **Bold** |
| `dim(s)` | Dimmed |
| `italic(s)` | *Italic* |
| `underline(s)` | Underlined |
| `strike(s)` | ~~Strikethrough~~ |

### Backgrounds

| Function | Effect |
|----------|--------|
| `bg_red(s)` | Red background |
| `bg_green(s)` | Green background |
| `bg_yellow(s)` | Yellow background |
| `bg_blue(s)` | Blue background |

### Advanced

| Function | Returns | Description |
|----------|---------|-------------|
| `rgb(s, r, g, b)` | `str` | 24-bit true color foreground |
| `bg_rgb(s, r, g, b)` | `str` | 24-bit true color background |
| `strip(s)` | `str` | Remove all ANSI codes |

## Usage Example

```desi
import color

def main() -> int:
    print(color.red("Error: file not found"))
    print(color.bold(color.green("✓ All tests passed")))
    print(color.yellow("⚠ Warning: deprecated API"))

    # Combine styles
    print(color.bold(color.underline("Important")))

    # True color (RGB)
    print(color.rgb("Custom color!", 255, 128, 0))
    0
```

## See Also

- [Log Module](log.md) — Logging with levels

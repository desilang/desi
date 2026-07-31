# fmt — String & Number Formatting

Advanced formatting beyond f-strings: padding, alignment, number formatting, currency, and more.

## Import

```desi
import fmt
```

## API Reference

### String Alignment

| Function | Description | Example |
|---|---|---|
| `fmt.pad_left(s, width, fill)` | Left-pad | `pad_left("hi", 10, ".")` → `"........hi"` |
| `fmt.pad_right(s, width, fill)` | Right-pad | `pad_right("hi", 10, ".")` → `"hi........"` |
| `fmt.center(s, width, fill)` | Center | `center("hi", 10, "-")` → `"----hi----"` |
| `fmt.truncate(s, max)` | Truncate with `...` | `truncate("hello world", 8)` → `"hello..."` |

### Number Formatting

| Function | Description | Example |
|---|---|---|
| `fmt.comma(n)` | Comma separators | `comma(1234567)` → `"1,234,567"` |
| `fmt.fixed(val, dec)` | Fixed decimals | `fixed(3.14159, 2)` → `"3.14"` |
| `fmt.percent(val, dec)` | Percentage | `percent(0.857, 1)` → `"85.7%"` |
| `fmt.currency(val, sym)` | Currency | `currency(1234.5, "$")` → `"$1,234.50"` |
| `fmt.ordinal(n)` | Ordinal | `ordinal(3)` → `"3rd"` |
| `fmt.bytes(n)` | Human-readable size | `bytes(1536)` → `"1.5 KB"` |

### String Utilities

| Function | Description | Example |
|---|---|---|
| `fmt.repeat(s, n)` | Repeat string | `repeat("ha", 3)` → `"hahaha"` |
| `fmt.reverse(s)` | Reverse string | `reverse("hello")` → `"olleh"` |
| `fmt.join(sep, list)` | Join list | `join(", ", ["a","b"])` → `"a, b"` |

## Examples

### Formatting a Table

```desi
import fmt

print(fmt.pad_right("Name", 20, " ") + fmt.pad_right("Price", 10, " "))
print(fmt.repeat("-", 30))
print(fmt.pad_right("Widget", 20, " ") + fmt.currency(29.99, "$"))
print(fmt.pad_right("Gadget", 20, " ") + fmt.currency(149.50, "$"))
```

### Report

```desi
import fmt

let total = 1234567
let growth = 0.156
print(f"Revenue: {fmt.currency(1234567.0, "$")}")
print(f"Growth: {fmt.percent(growth, 1)}")
print(f"Orders: {fmt.comma(total)}")
print(f"Rank: {fmt.ordinal(1)}")
print(f"Storage: {fmt.bytes(1073741824)}")
```

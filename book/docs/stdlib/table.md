# table — Pretty Table Printing

Render formatted ASCII tables for CLI output.

## Import

```desi
import table
```

## API Reference

| Function | Description |
|---|---|
| `table.new(columns) -> cptr` | Create a table with column headers |
| `table.add_row(h, values) -> int` | Add a data row |
| `table.set_align(h, col, align) -> int` | Set column alignment (0=left, 1=right, 2=center) |
| `table.size(h) -> int` | Number of data rows |
| `table.render(h) -> str` | Render with Unicode box-drawing characters |
| `table.render_simple(h) -> str` | Render with ASCII borders |
| `table.free(h) -> int` | Free the table |

## Examples

### Basic Table

```desi
import table

def main() -> int:
    let t = table.new(["Name", "Age", "Language"])
    table.add_row(t, ["Alice", "30", "Desi"])
    table.add_row(t, ["Bob", "25", "Rust"])
    table.add_row(t, ["Charlie", "35", "Python"])

    print(table.render_simple(t))
    table.free(t)
    0
```

Output:

```
+---------+-----+----------+
| Name    | Age | Language |
+---------+-----+----------+
| Alice   | 30  | Desi     |
| Bob     | 25  | Rust     |
| Charlie | 35  | Python   |
+---------+-----+----------+
```

### Unicode Borders

```desi
import table

def main() -> int:
    let t = table.new(["Status", "Count"])
    table.add_row(t, ["Passed", "42"])
    table.add_row(t, ["Failed", "3"])
    table.add_row(t, ["Skipped", "1"])

    print(table.render(t))
    table.free(t)
    0
```

Output:

```
┌─────────┬───────┐
| Status  | Count |
├─────────┼───────┤
| Passed  | 42    |
| Failed  | 3     |
| Skipped | 1     |
└─────────┴───────┘
```

# csv — CSV Parsing and Writing

The `csv` module provides RFC 4180 compliant CSV parsing and writing.

## Import

```desi
import csv
```

## API Reference

| Function | Description |
|---|---|
| `csv.parse(data: str) -> list[list[str]]` | Parse CSV string into rows |
| `csv.parse_delim(data: str, delim: str) -> list[list[str]]` | Parse with custom delimiter |
| `csv.format(rows: list[list[str]]) -> str` | Format rows into CSV string |
| `csv.read_file(path: str) -> list[list[str]]` | Read CSV file into rows |
| `csv.write_file(path: str, rows: list[list[str]]) -> int` | Write rows to CSV file (0=ok) |

## Examples

### Parse a CSV string

```desi
import csv

let data = "name,age,city\nAlice,30,NYC\nBob,25,LA"
let rows = csv.parse(data)

let header = rows[0]
print(header[0])  # name
print(header[1])  # age

let row = rows[1]
print(row[0])     # Alice
print(row[1])     # 30
```

### Write CSV to file

```desi
import csv

let rows = [["name", "score"], ["Alice", "95"], ["Bob", "87"]]
csv.write_file("/tmp/scores.csv", rows)
```

### Custom delimiter (TSV)

```desi
import csv

let tsv_data = "name\tage\nAlice\t30"
let rows = csv.parse_delim(tsv_data, "\t")
```

## Features

- **RFC 4180 compliant** — handles quoted fields, escaped quotes (`""`), commas and newlines inside quoted fields
- **Read/write files** directly with `read_file` and `write_file`
- **Custom delimiters** for TSV and other variants

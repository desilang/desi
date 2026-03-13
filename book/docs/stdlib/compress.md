# compress — gzip/zlib Compression

Compress and decompress strings and files using the zlib library.

## Import

```desi
import compress
```

## API Reference

| Function | Description |
|---|---|
| `compress.deflate(input) -> str` | Compress string (returns hex-encoded data) |
| `compress.inflate(data) -> str` | Decompress hex data back to string |
| `compress.ratio(orig, comp) -> int` | Compression ratio as percentage |
| `compress.gzip_file(in, out) -> int` | gzip compress a file |
| `compress.gunzip_file(in, out) -> int` | Decompress a gzip file |

## Examples

### String Compression

```desi
import compress

let data = "Hello World! " * 100
let compressed = compress.deflate(data)
let restored = compress.inflate(compressed)
assert restored == data

let ratio = compress.ratio(data, compressed)
print(f"Compressed to {str(ratio)}% of original")
```

### File Compression

```desi
import compress

compress.gzip_file("data.txt", "data.txt.gz")
compress.gunzip_file("data.txt.gz", "data_restored.txt")
```

> **Note:** Short strings may not compress well (zlib header overhead).
> Compression is most effective on larger, repetitive data.

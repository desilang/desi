# Decimal Library (libmpdec)

This directory contains **libmpdec 4.0.1**, a high-performance arbitrary precision
decimal floating-point library used for Desi's `decimal` type.

## License

BSD Simplified License - see [COPYRIGHT.txt](COPYRIGHT.txt)

## Pre-built Library

The `lib/` and `include/` directories contain pre-built artifacts for **macOS ARM64**.

For other platforms, you must build from source.

## Building from Source

### Prerequisites
- C compiler (gcc/clang)
- make

### Unix/Linux/macOS

```bash
cd mpdecimal-4.0.1
./configure
make -C libmpdec
cp libmpdec/libmpdec.a ../lib/
cp libmpdec/mpdecimal.h ../include/
```

### Windows

```batch
cd mpdecimal-4.0.1\vcbuild
vcbuild64.bat
copy ..\libmpdec\libmpdec.lib ..\lib\
copy ..\libmpdec\mpdecimal.h ..\include\
```

## Desi Integration

The wrapper `desi_decimal.c` provides simplified functions:

| Function | Description |
|----------|-------------|
| `__decimal_new(str)` | Create from string |
| `__decimal_from_int(i64)` | Create from integer |
| `__decimal_add/sub/mul/div` | Arithmetic |
| `__decimal_cmp` | Compare (-1, 0, 1) |
| `__decimal_to_str` | Convert to string |
| `__decimal_free` | Free memory |

## Memory Safety

Decimal values are heap-allocated. Desi's compiler ensures:
- Automatic cleanup via scope-based deallocation
- No use-after-free through ownership tracking

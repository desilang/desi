# YAML Module Implementation

Internal documentation for the `yaml` standard library module.

## Architecture

```
compiler/lib/yaml.desi     → Desi API (parse, query)
compiler/runtime/yaml.c    → C runtime (YAML parser)
```

## C Runtime Symbols

| C Symbol | Desi API | Signature |
|----------|----------|-----------|
| `__yaml_parse` | `yaml.parse` | `(const char*) → YamlDoc*` |
| `__yaml_parse_file` | `yaml.parse_file` | `(const char*) → YamlDoc*` |
| `__yaml_get` | `yaml.get` | `(YamlDoc*, char*) → char*` |
| `__yaml_get_int` | `yaml.get_int` | `(YamlDoc*, char*) → int32` |
| `__yaml_get_bool` | `yaml.get_bool` | `(YamlDoc*, char*) → int32` (→ bool) |
| `__yaml_has` | `yaml.has` | `(YamlDoc*, char*) → int32` (→ bool) |
| `__yaml_keys` | `yaml.keys` | `(YamlDoc*, char*) → char*` (comma-separated) |
| `__yaml_len` | `yaml.list_len` | `(YamlDoc*, char*) → int32` |
| `__yaml_dump` | `yaml.dump` | `(YamlDoc*) → char*` |
| `__yaml_free` | `yaml.free` | `(YamlDoc*) → int32` |

## Implementation Notes

- The YAML parser supports a subset of YAML 1.2: mappings, sequences, scalars.
- Dot-path notation (e.g. `"database.host"`) is used for nested access.
- Array elements are accessed via numeric indices in the path (e.g. `"servers.0.name"`).
- `keys()` returns comma-separated strings.
- Handles are `cptr` — opaque pointers to `YamlDoc` structs.

## Test Coverage

- `examples/511_yaml_module.desi`

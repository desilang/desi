# JSON Stdlib Implementation

This document provides implementation details for the `json` stdlib module for contributors.

## Architecture

The JSON module uses the **hybrid C+Desi pattern** (see [stdlib_architecture.md](stdlib_architecture.md)):

```
┌─────────────────────────────────────────┐
│  User Code         import json          │
│                    json.parse(text)     │
└─────────────────────────────────────────┘
              │
              ▼
┌─────────────────────────────────────────┐
│  compiler/lib/json.desi                 │
│  - pub enum JsonValue                   │
│  - pub def parse(text) -> JsonValue     │
│  - @extern("C") bindings                │
└─────────────────────────────────────────┘
              │
              ▼
┌─────────────────────────────────────────┐
│  compiler/runtime/json.c                │
│  - __json_parse() - fast C parser       │
│  - __json_stringify() - serialization   │
│  - __json_get_*() - value extractors    │
└─────────────────────────────────────────┘
```

## Files

| File | Purpose |
|------|---------|
| `compiler/runtime/json.c` | C runtime (~500 lines) |
| `compiler/lib/json.desi` | Desi wrapper with safe types |

## C Runtime Functions

```c
// Parsing
JsonNode* __json_parse(const char* text);
void __json_free(JsonNode* node);

// Type checking  
int __json_type(JsonNode* node);  // Returns JSON_NULL/BOOL/NUMBER/STRING/ARRAY/OBJECT

// Value extraction
int __json_get_bool(JsonNode* node);
double __json_get_number(JsonNode* node);
const char* __json_get_string(JsonNode* node);

// Array access
int __json_array_len(JsonNode* node);
JsonNode* __json_array_get(JsonNode* node, int index);

// Object access
int __json_object_len(JsonNode* node);
const char* __json_object_key(JsonNode* node, int index);
JsonNode* __json_object_get(JsonNode* node, const char* key);

// Serialization
char* __json_stringify(JsonNode* node);
```

## Desi API

```desi
pub enum JsonValue:
    Null: none
    Bool: bool
    Number: float
    String: str

pub def parse(text: str) -> JsonValue
pub def stringify(value: JsonValue) -> str  # TODO
```

## Adding New Functions

1. **Add C function** in `json.c` with `__json_` prefix
2. **Add @extern binding** in `json.desi`
3. **Add Desi wrapper** that calls extern and wraps safely
4. **Add test** in `examples/` with `EXPECTED_OUTPUT`
5. **Update docs** in `book/docs/language/json.md`

## Type Mapping

| JSON Type | C Type | Desi Type |
|-----------|--------|-----------|
| null | JSON_NULL (0) | JsonValue.Null |
| true/false | JSON_BOOL (1) | JsonValue.Bool |
| number | JSON_NUMBER (2) | JsonValue.Number |
| string | JSON_STRING (3) | JsonValue.String |
| array | JSON_ARRAY (4) | TODO: list[JsonValue] |
| object | JSON_OBJECT (5) | TODO: dict[str, JsonValue] |

## TODO

- [ ] `stringify()` - Convert JsonValue to JSON string
- [ ] `get(key)` - Access object fields safely
- [ ] `array_get(index)` - Access array elements safely
- [ ] Array/Object variants in JsonValue enum
- [ ] Result[JsonValue, str] error handling

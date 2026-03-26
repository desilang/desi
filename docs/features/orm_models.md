# ORM @model Decorator

**Status**: ✅ Implemented  
**Since**: v0.10  
**Related**: [Keyword Arguments](kwargs.md), [Classes](classes.md)

---

## Overview

The `@model("tablename")` decorator transforms a class declaration into a database model. The compiler:

1. Detects the decorator and marks the class as a model
2. Resolves ORM field type annotations (e.g., `CharField(100)`) into `OrmField` descriptors
3. Auto-inserts an `id: AutoField` primary key if none exists
4. Generates registration calls that run before `main()`
5. The C runtime handles SQL generation based on the selected dialect

## Syntax

```desi
@model("users")
class User:
    pub name: CharField(100, unique=true)
    pub author_id: ForeignKey(User, on_delete=CASCADE)
```

Both bracket `CharField[100]` and paren `CharField(100, unique=true)` syntaxes are supported. Paren syntax adds keyword arguments support.

## Compiler Implementation

### Pipeline

```
Parser → AST → Type Checker → Lowerer → LLVM → C Runtime
```

### 1. AST (`ast/nodes.go`)

```go
type TypeName struct {
    Name       string
    Params     []*TypeName       // positional: CharField(100) → [{Name:"100"}]
    KwParams   []TypeNameKwArg   // keyword: unique=true → [{Key:"unique", Value:{Name:"true"}}]
    UnionTypes []*TypeName
    TupleTypes []*TypeName
    Span       diag.Span
}

type TypeNameKwArg struct {
    Key   string
    Value *TypeName
}
```

### 2. Parser (`parse/decl.go`)

`parseTypeName()` extended with `LPAREN` handling alongside existing `LBRACK`/`LT` for generic params:

```go
} else if p.accept(token.LPAREN) {
    // CharField(100, unique=true), ForeignKey(User, on_delete=CASCADE)
    for {
        if p.cur.Tok == token.IDENT && p.peek.Tok == token.ASSIGN {
            // keyword arg: on_delete=CASCADE
            kwParams = append(kwParams, ...)
        } else {
            // positional arg
            params = append(params, p.parseTypeConstructorArg())
        }
    }
}
```

`parseTypeConstructorArg()` handles `INT_DEC` (100, 200), `KW_true`/`KW_false`, and falls through to `parseTypeName()` for IDENTs.

### 3. Type Checker (`check/check_type.go`)

#### `collectClass()` — Decorator Detection

Detects `@model("tablename")` on class declarations. Sets `cls.IsModel = true` and `cls.TableName`.

#### `checkClass()` — Field Resolution

For each field in a `@model` class, calls `resolveOrmField()` which:

1. Matches the type name against 13 ORM field types
2. Reads positional `Params` (e.g., `max_length` for CharField)
3. Reads keyword `KwParams` (e.g., `on_delete=CASCADE`, `nullable=true`)
4. Returns an `OrmField` descriptor + inferred Desi type

Auto-PK: if no field has `PrimaryKey = true`, auto-inserts `{Name: "id", Kind: OrmAuto, PrimaryKey: true}`.

#### ForeignKey Resolution

```go
// ForeignKey(User, on_delete=CASCADE) → resolves class name to table
for _, node := range c.info.Types {
    if cls, ok := node.(*types.Class); ok && cls.IsModel && cls.Name == param.Name {
        refTable = cls.TableName      // "User" → "users"
        for _, mf := range cls.ModelFields {
            if mf.PrimaryKey { refColumn = mf.Name; break }
        }
    }
}
```

### 4. Types (`types/types.go`)

```go
type Class struct {
    // ... existing fields ...
    IsModel     bool        // @model decorator present
    TableName   string      // "users", "posts"
    ModelFields []*OrmField // field descriptors
}

type OrmField struct {
    Name       string
    Kind       OrmFieldKind  // OrmAuto, OrmChar, OrmForeignKey, etc.
    PrimaryKey bool
    MaxLength  int           // CharField
    Nullable   bool
    Unique     bool
    Default    string
    HasDefault bool          // distinguishes "no default" from "default=0"
    OnDelete   string        // ForeignKey: CASCADE, PROTECT, etc.
    RefTable   string        // ForeignKey: referenced table
    RefColumn  string        // ForeignKey: referenced column
    // + more options ...
}

type OrmMeta struct {
    UniqueConstraints [][]string  // [["title", "author_id"]]
    Indexes           [][]string  // [["title"]]
    Ordering          []string    // ["-created_at"]
    Abstract          bool
}
```

### 5. Lowerer (`lower/class_lower.go`, `lower/module_lower.go`)

`LowerModelInit()` generates an `__orm_init_ClassName` function that emits HIR calls:

```
__orm_model("tablename")
__orm_auto_field("id")
__orm_char_field("name", 100, 0, 0, 0)
__orm_foreign_key("author_id", "users", "id", "CASCADE", 0)
__orm_unique_constraint("title,author_id")   # from Meta
__orm_index("title")                         # from Meta
```

`module_lower.go` injects calls to all `__orm_init_*` functions at the start of `__top__`, ensuring model registration happens before `main()`.

### 6. C Runtime (`runtime/db_orm.c`)

- `__orm_model(table)` — creates a `ModelDef` entry
- `__orm_*_field(...)` — adds a `FieldDef` to the current model
- `__orm_unique_constraint(csv)` — registers UNIQUE constraint (comma-separated fields)
- `__orm_index(csv)` — registers INDEX (comma-separated fields)
- `__orm_create_table_sql(table)` — generates dialect-specific CREATE TABLE SQL with constraints
- `FieldDef.on_delete` — emits `ON DELETE CASCADE` etc. in FK constraints

### 7. Meta Class Pipeline

```
Parser: class Meta { unique_together: [title, author_id] }
    → parseTypeName() handles LBRACK → TypeName{Params: [{Name:"title"}, {Name:"author_id"}]}

Type Checker: extractMetaOptions() reads nested class Meta fields
    → cls.Meta.UniqueConstraints = [["title", "author_id"]]

Lowerer: LowerModelInit() emits constraint calls
    → __orm_unique_constraint("title,author_id")
    → __orm_index("title")

C Runtime: __orm_create_table_sql() appends
    → UNIQUE (title,author_id)
    → CREATE INDEX IF NOT EXISTS idx_posts_title ON posts (title)
```

> **Key fix**: `parseTypeName()` has an early guard (line ~228) that validates the current token. `token.LBRACK` must be included in this guard, or the `[items]` handler further down will never be reached.

### 8. Database Config (`desi.mod`)

`[database]` section in `desi.mod` → auto-configures SQL dialect:

```
Manifest parses [database].engine → "postgres" | "mysql"
  → emit_ir_cmd.go reads via project.FindRoot()
  → passes DbEngine to LowerModuleOptions
  → module_lower.go injects __orm_set_dialect() + __db_set_dialect()
     at start of __top__, BEFORE model init calls
```

**Validation** (in `manifest.go`):
- `engine`: required, must be `postgres` or `mysql`
- `schema_only = true`: skip connection field validation
- Without `schema_only`: `host`, `name`, `user` are required (DPM0008)

## Supported Keyword Arguments

| Key | Used By | Effect |
|-----|---------|--------|
| `on_delete` | ForeignKey | `ON DELETE CASCADE\|PROTECT\|SET_NULL\|...` |
| `nullable` | All | Omits `NOT NULL` |
| `unique` | All | Adds `UNIQUE` |
| `default` | IntField, BoolField | `DEFAULT value` |
| `max_length` | CharField | `VARCHAR(N)` |
| `auto_now_add` | DateTimeField | `DEFAULT NOW()` |
| `auto_now` | DateTimeField | Auto-update on save |
| `primary_key` | Any | Override auto PK |
| `db_index` | Any | Create index |

## File Map

| File | Role |
|------|------|
| `compiler/internal/ast/nodes.go` | `TypeName.KwParams`, `TypeNameKwArg` |
| `compiler/internal/parse/decl.go` | `LPAREN` in `parseTypeName()`, `parseTypeConstructorArg()` |
| `compiler/internal/types/types.go` | `Class.IsModel/TableName/ModelFields`, `OrmField` |
| `compiler/internal/check/check_type.go` | `@model` detection, `resolveOrmField()` |
| `compiler/internal/lower/class_lower.go` | `LowerModelInit()` |
| `compiler/internal/lower/module_lower.go` | `__top__` injection |
| `compiler/lib/db.desi` | Extern declarations for C runtime |
| `compiler/runtime/db_orm.c` | SQL generation, field registration |

## Testing

```bash
# Run ORM example
./bin/desic run examples/441_model_class.desi

# Full regression suite (437 tests)
bash test_examples.sh
```

## Future Work

- [x] Meta class (unique_together, indexes, ordering, abstract)
- [ ] Auto table creation (`db.create_tables()`)
- [ ] GeneratedField, CompositePK, choices
- [ ] Assignment syntax: `pub name = CharField(100)` alternative

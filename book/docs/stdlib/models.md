# ORM Models — Django-Style

Define your database schema as classes with the `@model` decorator. Desi auto-generates SQL, handles foreign keys, and registers models before `main()` runs — zero boilerplate.

## Import

```desi
import db
```

## Defining Models

```desi
import db

db.use_postgres()    # or db.use_mysql()

@model("users")
class User:
    pub name: CharField(100)
    pub email: CharField(200, unique=true)
    pub age: IntField(default=0)
    pub active: BoolField(default=true)
    pub created_at: DateTimeField(auto_now_add=true)
```

- `@model("tablename")` registers the class as a database table
- An `id` primary key is **auto-inserted** if you don't declare one
- Field types (`CharField`, `IntField`, etc.) are auto-injected inside `@model` classes — no imports needed

## Field Types

| Field | Syntax | SQL (Postgres) |
|-------|--------|----------------|
| `AutoField` | `pub id: AutoField` | `SERIAL PRIMARY KEY` |
| `BigAutoField` | `pub id: BigAutoField` | `BIGSERIAL PRIMARY KEY` |
| `CharField` | `pub name: CharField(100)` | `VARCHAR(100)` |
| `TextField` | `pub bio: TextField` | `TEXT` |
| `IntField` | `pub age: IntField(default=0)` | `INTEGER DEFAULT 0` |
| `BigIntField` | `pub count: BigIntField` | `BIGINT` |
| `BoolField` | `pub active: BoolField(default=true)` | `BOOLEAN DEFAULT TRUE` |
| `FloatField` | `pub score: FloatField` | `DOUBLE PRECISION` |
| `DecimalField` | `pub price: DecimalField(10, 2)` | `DECIMAL(10,2)` |
| `DateTimeField` | `pub created: DateTimeField(auto_now_add=true)` | `TIMESTAMPTZ DEFAULT NOW()` |
| `UUIDField` | `pub uid: UUIDField` | `UUID` |
| `JsonField` | `pub data: JsonField(nullable=true)` | `JSONB` |
| `ForeignKey` | `pub author_id: ForeignKey(User, on_delete=CASCADE)` | `INTEGER REFERENCES users(id) ON DELETE CASCADE` |

## Field Options

Pass options as keyword arguments:

```desi
@model("profiles")
class Profile:
    pub name: CharField(100, unique=true)           # UNIQUE constraint
    pub bio: TextField(nullable=true)               # allows NULL
    pub age: IntField(default=0)                    # DEFAULT 0
    pub email: CharField(200, db_index=true)        # CREATE INDEX
    pub uid: UUIDField(primary_key=true)            # use as PK instead of auto id
    pub created: DateTimeField(auto_now_add=true)   # DEFAULT NOW() on insert
    pub updated: DateTimeField(auto_now=true)       # auto-update on save
```

## ForeignKey

Reference another model by **class name** — Desi auto-resolves the table and primary key:

```desi
@model("users")
class User:
    pub name: CharField(100)

@model("posts")
class Post:
    pub title: CharField(200)
    pub author_id: ForeignKey(User, on_delete=CASCADE)
```

### on_delete Options

| Option | SQL | Behavior |
|--------|-----|----------|
| `CASCADE` | `ON DELETE CASCADE` | Delete related rows |
| `PROTECT` | App-level check | Prevent deletion |
| `SET_NULL` | `ON DELETE SET NULL` | Set FK to NULL |
| `SET_DEFAULT` | `ON DELETE SET DEFAULT` | Set FK to default |
| `DO_NOTHING` | `ON DELETE NO ACTION` | No action |
| `RESTRICT` | `ON DELETE RESTRICT` | Prevent if referenced |

## Auto Primary Key

If you don't declare any `AutoField` or `BigAutoField`, Desi auto-inserts:

```desi
@model("posts")
class Post:
    pub id: AutoField    # added automatically
```

To use a custom PK:

```desi
@model("sessions")
class Session:
    pub token: UUIDField(primary_key=true)    # no auto id
    pub user_id: ForeignKey(User, on_delete=CASCADE)
```

## Meta Class

Add a nested `class Meta` inside `@model` to declare multi-column constraints and indexes — just like Django:

```desi
@model("posts")
class Post:
    pub title: CharField(200)
    pub author_id: ForeignKey(User, on_delete=CASCADE)

    class Meta:
        unique_together: [title, author_id]
        indexes: [title]
```

### Meta Options

| Option | Syntax | SQL Output |
|--------|--------|------------|
| `unique_together` | `unique_together: [field1, field2]` | `UNIQUE (field1, field2)` in CREATE TABLE |
| `indexes` | `indexes: [field1]` | `CREATE INDEX idx_table_field1 ON table (field1)` |
| `ordering` | `ordering: [-created_at]` | *(query builder, not DDL)* |
| `abstract` | `abstract: true` | *(skip table generation)* |

- Fields are listed in brackets: `[field1, field2]`
- Prefix with `-` for descending order: `[-created_at]`
- Multiple constraints are supported

## Database Configuration

Configure your database engine in `desi.mod`:

```toml
# Schema-only (SQL generation, no live DB)
[database]
engine = "postgres"
schema_only = true

# Full connection
[database]
engine = "postgres"
host = "localhost"
name = "myapp_db"
user = "admin"
password = "secret"
ssl_mode = "require"
charset = "UTF8"
timezone = "UTC"
prefix = "app1_"
```

### Required Fields

| Field | When Required | Description |
|-------|---------------|-------------|
| `engine` | Always | `"postgres"` or `"mysql"` |
| `host` | Not schema_only | Hostname or IP (`"localhost"`, `"192.168.1.100"`) |
| `name` | Not schema_only | Database name |
| `user` | Not schema_only | DB username |

### Optional Fields

| Field | Default | Description |
|-------|---------|-------------|
| `schema_only` | `false` | If `true`, skip connection fields (SQL generation only) |
| `port` | `5432` / `3306` | Auto-defaults per engine |
| `password` | — | DB password |
| `ssl_mode` | — | `"disable"`, `"require"`, `"verify-ca"`, `"verify-full"` |
| `charset` | — | Character encoding (`"utf8mb4"`, `"UTF8"`) |
| `timezone` | — | Connection timezone (`"UTC"`, `"America/Chicago"`) |
| `prefix` | — | Table name prefix for multi-tenancy |
| `options` | — | Extra DSN/connection string parameters |

The compiler auto-configures the SQL dialect — no manual `db.use_postgres()` needed

## Choices (CHECK Constraints)

Add `choices=` to any field to generate a SQL `CHECK` constraint:

```desi
@model("articles")
class Article:
    pub status: CharField(20, choices=[draft, published, archived])
```

Generates: `status VARCHAR(20) NOT NULL CHECK (status IN ('draft', 'published', 'archived'))`

## Generated Fields (Computed Columns)

Use `GeneratedField` for columns computed from other fields:

```desi
@model("people")
class Person:
    pub first_name: CharField(50)
    pub last_name: CharField(50)
    pub full_name: GeneratedField(expression="first_name || ' ' || last_name", output=CharField(100))
```

Generates: `full_name VARCHAR(100) GENERATED ALWAYS AS (first_name || ' ' || last_name) STORED`

## Composite Primary Keys

Use `primary_key` in `class Meta` for multi-column primary keys (no auto `id` field):

```desi
@model("order_items")
class OrderItem:
    pub order_id: IntField()
    pub product_id: IntField()
    pub quantity: IntField(default=1)

    class Meta:
        primary_key: [order_id, product_id]
```

Generates: `PRIMARY KEY (order_id,product_id)` — the auto `id` field is omitted.

## Generating SQL

```desi
import db

def main() -> int:
    let sql = db.create_table_sql("users")
    print(sql)
    # CREATE TABLE IF NOT EXISTS users (
    #   id SERIAL PRIMARY KEY,
    #   name VARCHAR(100) NOT NULL,
    #   email VARCHAR(200) NOT NULL UNIQUE,
    #   age INTEGER NOT NULL DEFAULT 0,
    #   active BOOLEAN NOT NULL DEFAULT TRUE,
    #   created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
    # )
    0
```

## Database Dialect

Set the SQL dialect before printing/executing SQL:

```desi
db.use_postgres()    # PostgreSQL (default)
db.use_mysql()       # MySQL
```

## Full Example

```desi
import db

db.use_postgres()

@model("users")
class User:
    pub name: CharField(100)
    pub email: CharField(200, unique=true)
    pub age: IntField(default=0)
    pub active: BoolField(default=true)
    pub created_at: DateTimeField(auto_now_add=true)

@model("posts")
class Post:
    pub title: CharField(200)
    pub body: TextField(nullable=true)
    pub author_id: ForeignKey(User, on_delete=CASCADE)

    class Meta:
        unique_together: [title, author_id]
        indexes: [title]

def main() -> int:
    print(db.create_table_sql("users"))
    print("---")
    print(db.create_table_sql("posts"))
    0
```

## Model.objects — QuerySet Manager

Every `@model` class gets an auto-generated `objects` manager for Django-style CRUD:

```desi
# Create
User.objects.create(name="Ali", age="25", email="ali@example.com")

# Read
User.objects.all()                    # SELECT * FROM users
User.objects.filter(age__gt="18")     # SELECT * WHERE age > 18
User.objects.get(id="1")              # SELECT * WHERE id=1 LIMIT 1
User.objects.first()                  # SELECT * LIMIT 1
User.objects.last()                   # SELECT * ORDER BY id DESC LIMIT 1

# Update / Delete
User.objects.update(status="active")  # UPDATE users SET status='active'
User.objects.delete()                 # DELETE FROM users

# Aggregations
User.objects.count()                  # SELECT COUNT(*)
User.objects.exists()                 # Returns 1 if any rows exist

# Ordering
User.objects.order_by("-created_at")  # ORDER BY created_at DESC

# Exclude
User.objects.exclude(status="banned") # WHERE NOT (status = 'banned')
```

### Django-Style Lookups

Filter and exclude support double-underscore lookups:

| Lookup | SQL | Example |
|--------|-----|---------|
| `exact` (default) | `=` | `name="Ali"` |
| `__gt` | `>` | `age__gt="18"` |
| `__gte` | `>=` | `age__gte="21"` |
| `__lt` | `<` | `age__lt="65"` |
| `__lte` | `<=` | `age__lte="30"` |
| `__contains` | `LIKE '%val%'` | `name__contains="li"` |
| `__startswith` | `LIKE 'val%'` | `name__startswith="A"` |
| `__endswith` | `LIKE '%val'` | `name__endswith="i"` |
| `__isnull` | `IS NULL` | `email__isnull="true"` |
| `__in` | `IN (...)` | `status__in="1,2,3"` |

### Method Chaining

Chain methods for complex queries:

```desi
# Filter + order + limit
User.objects.filter(age__gt="18").order_by("-name").first()

# Filter + exclude + count
User.objects.filter(status="active").exclude(role="admin").count()

# All + order + first
User.objects.all().order_by("created_at").first()
```

### Q Objects — Complex Lookups

Use `Q()` for OR conditions and complex boolean logic:

```desi
# OR: status is active OR age > 21
User.objects.filter(Q(status="active") | Q(age__gt="21"))

# AND: status is active AND name is Ali
User.objects.filter(Q(status="active") & Q(name="Ali"))

# Complex: age > 18 OR (age < 10 AND status is active)
User.objects.filter(Q(age__gt="18") | Q(age__lt="10") & Q(status="active"))
```

Operator precedence: `&` (AND) binds tighter than `|` (OR), matching Python/Django behavior.

## Advanced QuerySet Methods

### UPSERT (ON CONFLICT)

Insert-or-update with conflict resolution:

```desi
# Set the conflict detection column, then insert fields
db.objects("users")
db.on_conflict("email")                 # conflict column
db.set_field("name", "Alice")
db.set_field("email", "alice@example.com")
db.do_upsert()
# PG:    INSERT INTO users (name,email) VALUES ($1,$2) ON CONFLICT (email) DO UPDATE SET name=$1,email=$2
# MySQL: INSERT INTO users (name,email) VALUES (?,?) ON DUPLICATE KEY UPDATE name=VALUES(name),email=VALUES(email)
```

### update_or_create

Find-and-update or create a new row — Django's `update_or_create()`:

```desi
db.objects("users")
db.filter("name", "=", "Alice")        # lookup fields
db.set_field("name", "Alice")          # all fields for insert/update
db.set_field("email", "alice@new.com")
let result = db.update_or_create()
# Returns: 1 (created), 0 (updated), -1 (error)
```

### JSON Field Helpers

Read and write individual keys inside JSON/JSONB columns:

```desi
# Set a key inside a JSON column
db.objects("users")
db.filter("id", "=", "42")
db.json_set("settings", "theme", "dark")
# PG:    UPDATE users SET settings = jsonb_set(settings, '{theme}', '"dark"') WHERE id = $1
# MySQL: UPDATE users SET settings = JSON_SET(settings, '$.theme', 'dark') WHERE id = ?

# Get a key from a JSON column
db.objects("users")
db.filter("id", "=", "42")
let theme = db.json_get("settings", "theme")
# PG:    SELECT settings->>'theme' FROM users WHERE id = $1 LIMIT 1
# MySQL: SELECT JSON_UNQUOTE(JSON_EXTRACT(settings, '$.theme')) FROM users WHERE id = ? LIMIT 1
```

### Window Functions

Add window function expressions to your SELECT:

```desi
db.objects("employees")
db.window("ROW_NUMBER()", "PARTITION BY dept ORDER BY salary DESC", "row_num")
db.fetch_all()
# SELECT *, ROW_NUMBER() OVER (PARTITION BY dept ORDER BY salary DESC) AS row_num FROM employees
```

Supported functions: `ROW_NUMBER()`, `RANK()`, `DENSE_RANK()`, `LAG()`, `LEAD()`, `SUM()`, `AVG()`, etc.

### Server-Side Cursors

Process large result sets in batches without loading everything into memory:

```desi
db.begin()                            # PG requires cursors inside transactions
db.objects("audit_logs")
db.cursor_declare("log_cursor")
while db.cursor_fetch(100) > 0:       # fetch 100 rows at a time
    # process batch...
    pass
db.cursor_close()
db.commit()
# PG:    DECLARE log_cursor CURSOR FOR SELECT * FROM audit_logs
#        FETCH 100 FROM log_cursor
# MySQL: SELECT * FROM audit_logs LIMIT 100 OFFSET 0 (auto-paginated)
```

### CTEs (Common Table Expressions)

Build `WITH` clauses for complex queries:

```desi
# Simple CTE
db.objects("active_users")
db.cte("active_users", "SELECT * FROM users WHERE is_active = true")
let rows = db.cte_fetch()
# WITH active_users AS (SELECT * FROM users WHERE is_active = true) SELECT * FROM active_users

# Recursive CTE (hierarchical data)
db.cte("tree", "SELECT id, parent_id, name FROM categories WHERE parent_id IS NULL " +
    "UNION ALL SELECT c.id, c.parent_id, c.name FROM categories c JOIN tree t ON c.parent_id = t.id")
db.cte_recursive()
db.objects("tree")
let rows = db.cte_fetch()
# WITH RECURSIVE tree AS (...) SELECT * FROM tree
```

### Subqueries

Use `EXISTS`, `NOT EXISTS`, `IN`, and `NOT IN` with nested queries:

```desi
# EXISTS: find users who have orders
db.objects("users")
let sub = db.subquery("orders", "1", "orders.user_id = users.id")
db.filter_exists(sub)
db.fetch_all()
# SELECT * FROM users WHERE EXISTS (SELECT 1 FROM orders WHERE orders.user_id = users.id)

# IN: find users with high-value orders
db.objects("users")
db.filter_in_subquery("id", db.subquery("orders", "user_id", "total > 100"))
db.fetch_all()
# SELECT * FROM users WHERE id IN (SELECT user_id FROM orders WHERE total > 100)

# NOT EXISTS / NOT IN also available:
db.filter_not_exists(sub)
db.filter_not_in_subquery("id", db.subquery("banned_users", "user_id", ""))
```

### prefetch_related (N+1 Optimization)

Batch-fetch related rows to avoid N+1 query problems:

```desi
db.objects("posts")
db.prefetch_related("comments", "post_id", "id")  # related_table, fk_col, pk_col
let rows = db.prefetch_execute()
# 1. SELECT * FROM posts
# 2. SELECT * FROM comments WHERE post_id IN ('1','2','3',...)  (batch query)
```

### JSON_TABLE (PG 17 / MySQL 8)

Transform JSON columns into relational rows:

```desi
db.objects("orders")
let rows = db.json_table("data", "$.items[*]",
    "name TEXT PATH '$.name', qty INT PATH '$.qty'", "jt")
# PG:    SELECT jt.* FROM orders, json_table(data, '$.items[*]' COLUMNS (name TEXT PATH '$.name', qty INT PATH '$.qty')) AS jt
# MySQL: SELECT jt.* FROM orders, JSON_TABLE(data, '$.items[*]' COLUMNS (name TEXT PATH '$.name', qty INT PATH '$.qty')) AS jt
```

### Full-Text Search

Search text columns using native database full-text capabilities:

```desi
# Filter by full-text search
db.objects("articles")
db.fts_filter("body", "machine & learning", "english")
db.fetch_all()
# PG:    WHERE to_tsvector('english', body) @@ to_tsquery('english', 'machine & learning')
# MySQL: WHERE MATCH(body) AGAINST ('machine & learning' IN BOOLEAN MODE)

# Ranked search results
db.objects("articles")
db.fts_filter("body", "deep learning", "english")
db.fts_rank("body", "deep learning", "english", "relevance")
db.order_by("-relevance")
db.fetch_all()
# PG:    SELECT *, ts_rank(to_tsvector('english', body), to_tsquery('english', 'deep learning')) AS relevance ... ORDER BY relevance DESC

# Create a full-text index
db.fts_create_index("articles", "body", "idx_articles_body_fts", "english")
# PG:    CREATE INDEX idx_articles_body_fts ON articles USING gin(to_tsvector('english', body))
# MySQL: ALTER TABLE articles ADD FULLTEXT INDEX idx_articles_body_fts(body)
```

### Vector Similarity Search (pgvector / MySQL 9)

Find nearest neighbors using vector embeddings:

```desi
# Search for the 10 most similar documents
db.objects("documents")
db.vector_search("embedding", "[0.1, 0.2, 0.3]", "cosine", 10)
db.fetch_all()
# PG:    SELECT * FROM documents ORDER BY embedding <=> '[0.1, 0.2, 0.3]' LIMIT 10
# MySQL: SELECT * FROM documents ORDER BY DISTANCE(embedding, TO_VECTOR('[0.1, 0.2, 0.3]'), 'COSINE') LIMIT 10
```

**Distance metrics:**

| Metric | PG Operator | MySQL | Use Case |
|--------|-------------|-------|----------|
| `"cosine"` | `<=>` | `COSINE` | Normalized embeddings (most common) |
| `"l2"` | `<->` | `L2` | Euclidean distance |
| `"ip"` | `<#>` | `DOT` | Inner product |

```desi
# Create a vector index for fast similarity search
db.vector_create_index("documents", "embedding", "idx_docs_embedding", "cosine")
# PG:    CREATE INDEX idx_docs_embedding ON documents USING hnsw (embedding vector_cosine_ops)
# MySQL: ALTER TABLE documents ADD VECTOR INDEX idx_docs_embedding(embedding) DISTANCE = 'COSINE'
```

## See Also

- [Database & ORM](database.md) — CRUD operations, query builder, transactions
- [PostgreSQL](postgres.md) — PG-specific connection and types
- [MySQL](mysql.md) — MySQL-specific connection and types

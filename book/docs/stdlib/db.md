# db — Query Builder & ORM

Desi's database module provides a SQL query builder and ORM for defining models.

## Import

```desi
import db
```

## Dialect

Set the SQL dialect before building queries or generating DDL:

```desi
db.use_postgres()  # PostgreSQL (default)
db.use_mysql()     # MySQL
```

## Query Builder

### SELECT

```desi
db.find("users")
db.columns("name")
db.columns("email")
db.where("age", ">", "21")
db.order_by("name", 0)
db.limit(10)
let sql = db.build_sql()
# → SELECT name, email FROM users WHERE age > 21 ORDER BY name LIMIT 10
```

### INSERT

```desi
db.insert_into("users")
db.set_field("name", "Alice")
db.set_field("email", "alice@example.com")
let sql = db.build_sql()
# → INSERT INTO users (name, email) VALUES ('Alice', 'alice@example.com')
```

### UPDATE

```desi
db.update_table("users")
db.set_field("name", "Alice Smith")
db.where("id", "=", "1")
let sql = db.build_sql()
# → UPDATE users SET name = 'Alice Smith' WHERE id = 1
```

### DELETE

```desi
db.remove("users")
db.where("id", "=", "5")
let sql = db.build_sql()
# → DELETE FROM users WHERE id = 5
```

### JOIN

```desi
db.find("orders")
db.columns("orders.id")
db.columns("users.name")
db.join("INNER", "users", "users.id = orders.user_id")
let sql = db.build_sql()
```

## ORM — Model Definition

```desi
db.model("users")
db.auto_field("id")
db.char_field("name", 100, 0, 0)
db.char_field("email", 255, 0, 1)   # unique
db.int_field("age", 0, 0)
db.text_field("bio", 1)             # nullable
db.bool_field("active", 1, 0)
db.datetime_field("created_at", 0, 1, 0)  # auto_now_add
db.json_field("metadata", 1)

let sql = db.create_table_sql("users")
```

See [PostgreSQL](postgres.md) and [MySQL](mysql.md) for dialect-specific output.

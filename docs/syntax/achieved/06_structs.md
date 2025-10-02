
# 06 · Structs

Declare with fields; construct with field initializers; access and assign with `:=`.

```desi
struct User:
  id: int
  name: str

def main() -> int:
  let mut u: User = User{ id: 1, name: "Ada" }
  u.id := 42
  print("id:", u.id, "name:", u.name)
  0
```

Nested fields:

```desi
struct Name:
  first: str
  last: str

struct User:
  id: int
  name: Name

u.name.first := "Ada"
```


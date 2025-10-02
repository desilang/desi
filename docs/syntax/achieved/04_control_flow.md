
# 04 · Control Flow

### If / Elif / Else (block and single-line)

```desi
if x == 1:
  print(1)
elif x == 2:
  print(2)
else:
  print(0)
```

Single-line:

```desi
if x == 0: print(42)
```

### While (block and single-line)

```desi
let mut i: int = 0
while i < 3:
  print(i)
  i := i + 1
```

```desi
while i < 3: print(i)
```

### Defer (function-scope)

```desi
defer print("bye")
```


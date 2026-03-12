# template — Simple String Templates

The `template` module provides `{{key}}` placeholder substitution and HTML escaping.

## Import

```desi
import template
```

## API Reference

| Function | Description |
|---|---|
| `template.render(tmpl, keys, values)` | Render `{{key}}` placeholders |
| `template.render_pairs(tmpl, pairs)` | Render with `[k1, v1, k2, v2, ...]` list |
| `template.escape_html(input)` | Escape `&`, `<`, `>`, `"`, `'` |

## Examples

### Basic rendering

```desi
import template

let result = template.render(
    "Hello, {{name}}! Welcome to {{city}}.",
    ["name", "city"],
    ["Alice", "Wonderland"]
)
print(result)
# Hello, Alice! Welcome to Wonderland.
```

### Render with pairs

```desi
import template

let msg = template.render_pairs(
    "{{greeting}}, {{who}}!",
    ["greeting", "Hi", "who", "World"]
)
print(msg)  # Hi, World!
```

### Unknown keys are preserved

```desi
import template

let result = template.render(
    "{{known}} and {{missing}}",
    ["known"],
    ["found"]
)
print(result)  # found and {{missing}}
```

### HTML escaping

```desi
import template

let safe = template.escape_html("<script>alert('xss')</script>")
print(safe)
# &lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;
```

## Features

- **Mustache-style** `{{key}}` placeholders
- **Whitespace tolerant** — `{{ name }}` works too
- **Safe by default** — unknown keys preserved, not silently removed
- **HTML escaping** — prevent XSS in web templates

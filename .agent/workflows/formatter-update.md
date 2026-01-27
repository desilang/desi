---
description: Update formatter when adding new syntax/features
---

# Formatter Update Workflow

When implementing **new syntax or language features**, ensure the formatter handles them correctly.

## 1. Check Token Coverage

Review `compiler/internal/format/format.go`:

- **`isBinaryLike` map** (lines ~433-456) - binary operators needing spaces
- **`isFollowedBySpace` map** (lines ~458-478) - keywords needing trailing space
- **`shouldSpaceBefore()`** - main spacing logic

## 2. Add New Tokens

If adding new operators or keywords:

```go
// In isBinaryLike map (for binary operators):
token.NEW_OPERATOR: true,

// In isFollowedBySpace map (for keywords):
token.KW_new_keyword: true,
```

## 3. Run Formatter Tests

// turbo
```bash
go test ./compiler/internal/format/... -v
```

## 4. Test with Examples

// turbo
```bash
./bin/desifmt examples/your_new_feature.desi
```

## 5. Verify Idempotence

// turbo
```bash
# Format twice - output should be identical
./bin/desifmt examples/your_file.desi > /tmp/f1.desi
./bin/desifmt /tmp/f1.desi > /tmp/f2.desi
diff /tmp/f1.desi /tmp/f2.desi
```

## 6. (Optional) Add Golden Test

Create test file in `compiler/internal/format/testdata/`:
- `your_feature.input.desi` - unformatted input
- `your_feature.golden.desi` - expected output

## Quick Reference: Recent Syntax Additions

| Feature | Token | Formatter Status |
|---------|-------|------------------|
| `?` operator | `QUESTION` | ✅ Handled (postfix) |
| Turbofish `::` | `COLON_COLON` | ✅ Handled |
| Pipe `\|>` | `PIPE_GT` | ✅ Binary operator |
| Fat arrow `=>` | `FAT_ARROW` | ✅ Handled |

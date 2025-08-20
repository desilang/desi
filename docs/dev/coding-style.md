# Coding Style

## Go style (compiler)
- **Formatting**: `go fmt` (CI enforces).
- **Lints**: prefer `staticcheck` defaults; avoid stutters (`lexer.Token`, not `lexer.LexerToken`).
- **Panics**: the compiler should **not panic** on user input. Return diagnostics; panic only for truly impossible invariants.
- **Errors**: wrap with context (`fmt.Errorf("parser: %w", err)`). No naked `errors.New` for surfaced errors; use diagnostics.
- **Packages**: keep them small and cohesive (`lexer`, `parser`, `types`, `ir`, `lower`, `codegen/c`, `runtime`).
- **Naming**:
  - Packages: lower_snake (Go default is lower; avoid caps).
  - Types/funcs: `CamelCase`.
  - Locals: short, meaningful (`tok`, `pos`, `n`).
- **Interfaces**: define where they’re consumed. Don’t over-abstract early.
- **Context**: not needed yet; we’ll add when concurrent passes arrive.
- **Allocations**: prefer stack + slices; avoid heap unless necessary.
- **Safety**: never index without bounds checks unless proven safe.

## Repository conventions
- **Conventional Commits** for every PR.
- **Docs-first**: any user-visible change must update `docs/spec/*` and/or `docs/stages/*`.
- **Tests**: add or update tests in `tests/*` with each change.
- **ADRs/RFCs**: record significant decisions.

## Diagnostics
- Single source of truth in `compiler/internal/diag`.
- Diagnostics must include **line/col** and a short **human-readable** message.
- Prefer deterministic messages to keep golden tests stable.

## Comments & docs
- Package doc comment at the top of each package.
- Exported symbols have GoDoc comments.
- Keep examples small; longer narratives go to `docs/`.

## Performance notes
- Avoid reflection.
- Reuse buffers where practical.
- Keep hot loops branch-light; prefer table-driven logic in the lexer.

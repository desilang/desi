# Contributing to Desi

Thanks for your interest in contributing!

## Ground rules
- Use **Conventional Commits** (e.g., `feat(lexer): add numeric literals`).
- Any behavior or syntax change **must** update docs in `docs/spec/*` and/or `docs/stages/*`.
- Add/adjust tests under `tests/*` (golden tests welcome).
- Large changes require an **RFC** in `docs/rfcs/` before coding.
- Record notable decisions as **ADRs** in `docs/adr/`.

## Getting started
1. Install Go 1.25.x.
2. Clone the repo and run:
   ```bash
   go run ./compiler/cmd/desic --help
   go test ./...
   ```

3. For Windows/macOS C backend work, install LLVM/Clang (later stages).

## Development flow

* Small PRs are easier to review.
* Keep commits focused; prefer multiple commits over one giant one.
* Update `docs/` and examples when user-visible behavior changes.

## Code style

* Go code is formatted with `go fmt`.
* Keep packages small and focused.
* Avoid global state; pass dependencies explicitly.

## Reporting issues

* Use the GitHub issue templates.
* Include OS, Go version, exact repro steps, and expected vs actual behavior.

## Security

See `SECURITY.md` for coordinated disclosure.

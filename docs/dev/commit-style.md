# Commit Style — Conventional Commits

We use **Conventional Commits** to structure history and automate changelogs.

## Format
```

<type>(optional scope): <description>

\[optional body]

\[optional footer(s)]

```

### Types (common)
- `feat`: a new feature (user-visible)
- `fix`: a bug fix
- `docs`: documentation changes
- `perf`: performance improvements
- `refactor`: code change that neither fixes a bug nor adds a feature
- `test`: adding or correcting tests
- `build`: build system or external dependency changes
- `ci`: CI configuration changes
- `chore`: other changes that don't modify src or tests

### Examples
- `feat(lexer): support hex and binary integer literals`
- `fix(parser): correct precedence of unary minus`
- `docs(spec): clarify match exhaustiveness rules`
- `ci: add GitHub Actions matrix for mac/win/linux`

### Footer keywords
- `Closes #123`, `Fixes #45` to auto-close issues.
- `BREAKING CHANGE:` in body or footer for breaking changes (rare in Stage-0).

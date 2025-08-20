# Governance

For Stage-0, Desi uses a **BDFL + Maintainers** model.

## Roles
- **BDFL**: Sets vision, final tie-breaker on disputes.
- **Maintainers**: Review PRs, triage issues, cut releases, and steward subsystems (lexer, parser, runtime, docs).
- **Contributors**: Anyone sending issues/PRs.

## Decision process
- Day-to-day decisions happen via PR review.
- Significant or user-visible changes require:
  1) an **RFC** under `docs/rfcs/`, and
  2) an **ADR** entry under `docs/adr/` after acceptance.
- Consensus is preferred. If consensus cannot be reached, BDFL decides.

## Changes that require an RFC
- Syntax changes
- Type system changes
- Runtime/ABI changes
- Tooling UX changes (`desic build/test/fmt`)
- Dependency/licensing changes

## Meetings
- Ad-hoc; decisions are recorded in ADRs and PR history.

## Maintainer conduct
- Follow the Code of Conduct.
- Be responsive in reviews; explain rejections with actionable guidance.

## Amendments
- This document may be updated via PR and maintainer approval.

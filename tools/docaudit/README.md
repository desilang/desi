# Documentation audit tooling

Compiles the Desi code blocks in `book/docs` so the documentation can be held
to the same standard as the examples. The last full run found that two of nine
complete programs on the newcomer path did not compile, and that the docs were
teaching `var` — which hung the compiler.

## Why these scripts and not a shell loop

`desic` is a native Windows binary, and **Git Bash's `timeout` does not
reliably kill one**. Two earlier runs let a runaway reach 2 GB and froze the
machine. `safecheck.ps1` kills by process handle and also aborts on a memory
ceiling, so a pathological input costs one file instead of the session.

Use these rather than looping over `desic` directly.

## Usage

```bash
# 1. Extract every ```desi block, dedent it, and write an index.
#    Dedenting matters: a block lifted from inside a function is uniformly
#    indented and fails at 1:1 — that alone accounted for 31 false positives.
bash tools/docaudit/extract-blocks.sh
```

```powershell
# 2. Check them all, bounded, separating parse errors from context failures.
powershell -File tools\docaudit\checkfrags.ps1 -Index "$env:TEMP\dblocks3\frags_dd.tsv"

# Or one file, when narrowing something down:
powershell -File tools\docaudit\safecheck.ps1 -File path\to\block.desi -TimeoutSec 10 -MaxMB 800
```

## Reading the output

A **parse error** is a real defect: the syntax is wrong whatever surrounds it.

A **context failure** is expected. Most blocks are a few lines lifted from
prose and reference things defined earlier on the page, so a type error says
nothing about the documentation.

Parse errors still need triage into three kinds:

- **real defects** — wrong syntax, fix the page
- **extraction artifacts** — bare match arms, bare class fields; correct in
  context, unparseable alone
- **schematic** — placeholders like `pub field_name: Type`; these should be
  tagged ` ```text ` so they are not presented as compilable

## Baseline

887 blocks · 164 complete programs · 723 fragments · **74 parse failures
remaining**, down from 124.

The remaining failures need triage into the three kinds above before they are
treated as a defect count — sampling showed all three are represented, and no
single construct accounts for more than two of them.

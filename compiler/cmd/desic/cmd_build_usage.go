package main

import "github.com/desilang/desi/compiler/internal/term"

func usageBuild() {
	term.Eprintln("usage: desic build [flags] <entry.desi>")
	term.Eprintln("\nGeneral:")
	term.Eprintln("  --use-desi-lexer           use the self-hosted Desi lexer via bridge (Go parser)")
	term.Eprintln("  --use-desi-parser          use an external Desi parser bridge (AST JSON)")
	term.Eprintln("  --use-desi-parser-auto     auto-build + cache bridge from examples/compiler/desi/parser.desi")
	term.Eprintln("  --parsebridge-bin <path>   path to external parser bridge when using --use-desi-parser")
	term.Eprintln("  --keep-bridge-tmp          keep gen/tmp bridge artifacts (debugging)")
	term.Eprintln("  --verbose                  verbose bridge logging")
	term.Eprintln("  --Werror                   treat warnings as errors")
	term.Eprintln("\nC compile/link (enabled by default):")
	term.Eprintln("  --no-cc                    only emit C (skip compiling)")
	term.Eprintln("  --cc-bin=<cc>              choose compiler (clang/gcc/cl). Alias: --cc=<cc>")
	term.Eprintln("  --cc-arg=<flag>            pass through a flag to the C compiler (repeatable)")
	term.Eprintln("  --runtime-dir=<path>       override path to runtime/c (auto-detected otherwise)")
	term.Eprintln("  --out=<name>               output executable name (default: entry basename)")
	term.Eprintln("\nImports & modules:")
	term.Eprintln("  • Builtins (print, len, …) are always in scope — not importable and not shadowable.")
	term.Eprintln("  • Project root is discovered via desi.conf (walked upward from the entry file).")
	term.Eprintln("    Imports resolve in this order: project → std → DESI_PATH.")
	term.Eprintln("  • Bare std modules are available as top-level imports: e.g. 'import math', 'from time import now'.")
	term.Eprintln("  • Module mapping accepts either a single file or a package directory:")
	term.Eprintln("      a.b.c  →  <root>/a/b/c.desi    or   <root>/a/b/c/mod.desi")
	term.Eprintln("    If both exist in the same root, the resolver emits an error.")
	term.Eprintln("  • No string/path imports; only dotted identifiers are allowed (e.g., 'import util.math').")
}

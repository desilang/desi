package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/backend/llvm"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/lower"
	"github.com/desilang/desi/compiler/internal/parse"
	"github.com/desilang/desi/compiler/internal/term"
)

// init() runs before main(). We intercept "emit-ir" here and exit after handling it.
// This avoids modifying the existing main.go.
func init() {
	if len(os.Args) < 2 || os.Args[1] != "emit-ir" {
		return
	}

	file, err := parseEmitArgs(os.Args[2:])
	if err != nil {
		term.Eprintln("emit-ir error:", err)
		term.Flush()
		os.Exit(2)
	}

	src, err := os.ReadFile(file)
	if err != nil {
		term.Eprintln("emit-ir:", err)
		term.Flush()
		os.Exit(2)
	}

	// Parse (no resolve/check here; emit-ir is a Tier-0 demo tool)
	mod, diags := parse.ParseFile(file, src)
	if len(diags) > 0 {
		for _, d := range diags {
			term.Eprintln(d.RenderTTY(src))
		}
		term.Flush()
		os.Exit(2)
	}

	// Lower the entire module to HIR (sync: 1 fn; async: wrapper+poll)
	hm := lower.LowerModuleFromSource(mod, src)
	if hm == nil || len(hm.Funcs) == 0 {
		term.Eprintln("emit-ir:", filepath.Base(file)+": no functions to lower")
		term.Flush()
		os.Exit(2)
	}

	// Build textual LLVM module. Mark async wrapper names if their poll exists.
	lm := llvm.NewModule(filepath.Base(file))
	markAsyncWrappers(lm, hm)

	// Emit every lowered function (order as lowered is fine for Tier-0)
	for _, f := range hm.Funcs {
		lm.EmitFunc(f)
	}
	fmt.Print(lm.IR())
	term.Flush()
	os.Exit(0)
}

func parseEmitArgs(argv []string) (string, error) {
	// Very simple: one positional <file>. (We can add -I later if needed.)
	if len(argv) == 0 {
		return "", fmt.Errorf("usage: desic emit-ir <file.desi>")
	}
	// Ignore extra args for now; accept the first non-flag as the file.
	for _, a := range argv {
		if len(a) > 0 && a[0] != '-' {
			return a, nil
		}
	}
	return "", fmt.Errorf("missing <file.desi>")
}

func findMainFunc(m *ast.Module) *ast.FuncDecl {
	for _, d := range m.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok {
			if fd.Name.Name == "main" && fd.Body != nil {
				return fd
			}
		}
	}
	return nil
}

// markAsyncWrappers scans HIR for pairs "<name>" and "<name>$poll" and marks "<name>"
// as an async wrapper in the LLVM module so calls to it are emitted as 'call ptr'.
func markAsyncWrappers(lm *llvm.Module, hm *hir.Module) {
	seenPoll := map[string]bool{}
	for _, f := range hm.Funcs {
		if strings.HasSuffix(f.Name, "$poll") {
			base := strings.TrimSuffix(f.Name, "$poll")
			seenPoll[base] = true
		}
	}
	for _, f := range hm.Funcs {
		if seenPoll[f.Name] {
			lm.MarkAsyncWrapper(f.Name)
		}
	}
}

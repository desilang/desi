package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/backend/llvm"
	"github.com/desilang/desi/compiler/internal/check"
	"github.com/desilang/desi/compiler/internal/diag"
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
			// Render directly to stderr with a default (zero-value) theme.
			d.RenderTTY(os.Stderr, diag.Theme{})
		}
		term.Flush()
		os.Exit(2)
	}

	// NEW: run pre-check desugars so map/filter become list-comps
	// before lowering, which ensures compact IR without declare @map/filter.
	check.DesugarPrecheck(mod)

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

	// NEW: inject param/ret textual types from surface annotations.
	injectUserFuncSigs(mod)

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

// injectUserFuncSigs scans the AST for fn params/ret and registers textual LLVM types.
// Unrecognized/omitted types fall back to existing Tier-0 defaults (ptr/i32).
func injectUserFuncSigs(mod *ast.Module) {
	if mod == nil {
		return
	}
	for _, d := range mod.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		params := make([]string, len(fd.Params))
		for i := range fd.Params {
			if fd.Params[i].Type == nil {
				params[i] = "" // keep default (ptr)
				continue
			}
			params[i] = llvmTypeFromSurface(fd.Params[i].Type.Name)
		}
		ret := ""
		if fd.RetType != nil {
			ret = llvmTypeFromSurface(fd.RetType.Name)
		}
		// Package-level override (no Module struct changes needed).
		llvm.SetFuncSig(fd.Name.Name, ret, params)
	}
}

// Minimal surface->LLVM textual type mapping for Tier-0:
// - ints: i8..i128 → i8..i128
// - uints: u8..u128 → i8..i128 (2C ABI tier-0)
// - bool → i1
// - f32 → float; f64/float → double
// - usize/isize → i64 (Tier-0)
func llvmTypeFromSurface(name string) string {
	switch name {
	case "bool":
		return "i1"
	case "f32":
		return "float"
	case "f64", "float":
		return "double"
	case "usize", "isize":
		return "i64"
	case "str":
		return "ptr"
	case "i8", "i16", "i32", "i64", "i128":
		return name
	case "u8":
		return "i8"
	case "u16":
		return "i16"
	case "u32":
		return "i32"
	case "u64":
		return "i64"
	case "u128":
		return "i128"
	default:
		return "" // unknown → leave default
	}
}

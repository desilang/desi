package c

import (
	"fmt"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
)

// validateTypes scans function signatures for types the C backend doesn't support.
// We currently validate only function returns/params (lowest risk to AST coupling).
func validateTypes(f *ast.File) []string {
	var errs []string

	checkText := func(context, typ string) {
		t := strings.TrimSpace(strings.ToLower(typ))
		if t == "" {
			return
		}
		// Disallow 'void' entirely: use 'none' in Desi and we map that to C 'void'.
		if t == "void" {
			errs = append(errs, fmt.Sprintf("%s uses 'void' — use 'none' instead", context))
			return
		}
		// Unsupported numeric spellings in C backend.
		switch t {
		case "i8", "i16", "i64", "i128", "isize",
			"u8", "u16", "u64", "u128", "usize",
			"f32", "f64":
			errs = append(errs, fmt.Sprintf("%s uses unsupported type '%s' in C backend (use i32/u32/bool/str, or switch --backend=llvm)", context, t))
		}
	}

	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		// Return type
		checkText(fmt.Sprintf("function '%s' return type", fn.Name), fn.Ret)
		// Parameters
		for _, p := range fn.Params {
			checkText(fmt.Sprintf("parameter '%s' of function '%s'", p.Name, fn.Name), p.Type)
		}
	}

	return errs
}

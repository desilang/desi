package c

import (
	"fmt"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/check"
)

// validateForCBackend scans the AST for types the C backend cannot handle.
// It returns user-facing diagnostics to be emitted as `#error` lines.
func validateForCBackend(f *ast.File, info *check.Info) []string {
	var diags []string

	resolveAlias := func(t string) string {
		raw := strings.TrimSpace(t)
		for i := 0; i < 8; i++ {
			if info != nil && info.Types != nil {
				if u, ok := info.Types[raw]; ok {
					raw = strings.TrimSpace(u)
					continue
				}
			}
			break
		}
		return raw
	}

	// Returns category:
	// "none" | "int" | "str" | "future" | "struct" | "enum" | "unsupported:<t>" | "unknown:<t>"
	classify := func(t string) string {
		if t == "" {
			return "none"
		}
		raw := resolveAlias(t)
		lc := strings.ToLower(strings.TrimSpace(raw))

		if lc == "void" {
			return "unsupported:void"
		}

		switch lc {
		// supported builtins
		case "none":
			return "none"
		case "bool", "int", "i32", "u32",
			"i8", "i16", "i64", "isize",
			"u8", "u16", "u64", "usize":
			return "int"
		case "f32", "f64":
			return "int" // treat as supported scalar for validation purposes
		case "str", "string":
			return "str"
		case "future":
			return "future"

		// unsupported in C backend (LLVM-only)
		case "i128", "u128":
			return "unsupported:" + raw
		}

		if info != nil {
			if _, ok := info.Structs[raw]; ok {
				return "struct"
			}
			if info.Enums != nil {
				if _, ok := info.Enums[raw]; ok {
					return "enum"
				}
			}
		}
		return "unknown:" + raw
	}

	add := func(msg string) { diags = append(diags, msg) }

	for _, d := range f.Decls {
		switch v := d.(type) {
		case *ast.FuncDecl:
			switch c := classify(v.Ret); {
			case strings.HasPrefix(c, "unsupported:"):
				add(fmt.Sprintf("desic C backend: unsupported return type %q in function %q (use i8/i16/i32/i64/isize or u8/u16/u32/u64/usize, bool, str, f32, f64, none; otherwise use --backend=llvm)", strings.TrimPrefix(c, "unsupported:"), v.Name))
			case strings.HasPrefix(c, "unknown:"):
				add(fmt.Sprintf("desic C backend: unknown return type %q in function %q (define it as a struct/enum or use --backend=llvm)", strings.TrimPrefix(c, "unknown:"), v.Name))
			case c == "future" && !v.Async:
				add(fmt.Sprintf("desic C backend: non-async function %q cannot return 'future' (mark it 'async' or use --backend=llvm)", v.Name))
			}
			for _, p := range v.Params {
				switch c := classify(p.Type); {
				case c == "none":
					add(fmt.Sprintf("desic C backend: parameter %q of function %q cannot have type 'none'", p.Name, v.Name))
				case c == "future":
					add(fmt.Sprintf("desic C backend: parameter %q of function %q cannot have type 'future' (not supported by C backend)", p.Name, v.Name))
				case strings.HasPrefix(c, "unsupported:"):
					add(fmt.Sprintf("desic C backend: unsupported parameter type %q for %q in function %q (use i8/i16/i32/i64/isize or u8/u16/u32/u64/usize, bool, str, f32, f64; otherwise use --backend=llvm)",
						strings.TrimPrefix(c, "unsupported:"), p.Name, v.Name))
				case strings.HasPrefix(c, "unknown:"):
					add(fmt.Sprintf("desic C backend: unknown parameter type %q for %q in function %q (define it or use --backend=llvm)",
						strings.TrimPrefix(c, "unknown:"), p.Name, v.Name))
				}
			}

		case *ast.StructDecl:
			for _, ft := range v.Fields {
				switch c := classify(ft.Type); {
				case c == "none":
					add(fmt.Sprintf("desic C backend: field %q of struct %q cannot have type 'none'", ft.Name, v.Name))
				case c == "future":
					add(fmt.Sprintf("desic C backend: field %q of struct %q cannot have type 'future' (not supported by C backend)", ft.Name, v.Name))
				case strings.HasPrefix(c, "unsupported:"):
					add(fmt.Sprintf("desic C backend: unsupported field type %q for %q in struct %q (use i8/i16/i32/i64/isize or u8/u16/u32/u64/usize, bool, str, f32, f64; otherwise use --backend=llvm)",
						strings.TrimPrefix(c, "unsupported:"), ft.Name, v.Name))
				case strings.HasPrefix(c, "unknown:"):
					add(fmt.Sprintf("desic C backend: unknown field type %q for %q in struct %q (define it or use --backend=llvm)",
						strings.TrimPrefix(c, "unknown:"), ft.Name, v.Name))
				}
			}

		case *ast.EnumDecl:
			for _, ev := range v.Variants {
				trim := strings.TrimSpace(ev.Payload)
				if trim == "" || strings.EqualFold(trim, "none") {
					continue
				}
				switch c := classify(trim); {
				case c == "future":
					add(fmt.Sprintf("desic C backend: payload of %s.%s cannot be 'future' (not supported by C backend)", v.Name, ev.Name))
				case strings.HasPrefix(c, "unsupported:"):
					add(fmt.Sprintf("desic C backend: unsupported payload type %q for %s.%s (use i8/i16/i32/i64/isize or u8/u16/u32/u64/usize, bool, str, f32, f64; otherwise use --backend=llvm)",
						strings.TrimPrefix(c, "unsupported:"), v.Name, ev.Name))
				case strings.HasPrefix(c, "unknown:"):
					add(fmt.Sprintf("desic C backend: unknown payload type %q for %s.%s (define it or use --backend=llvm)",
						strings.TrimPrefix(c, "unknown:"), v.Name, ev.Name))
				}
			}

		case *ast.TypeDecl:
			switch c := classify(v.Underlying); {
			case c == "future":
				add(fmt.Sprintf("desic C backend: type alias %q cannot target 'future' (not supported by C backend)", v.Name))
			case strings.HasPrefix(c, "unsupported:"):
				add(fmt.Sprintf("desic C backend: type alias %q targets unsupported type %q (use i8/i16/i32/i64/isize or u8/u16/u32/u64/usize, bool, str, f32, f64; otherwise use --backend=llvm)",
					v.Name, strings.TrimPrefix(c, "unsupported:")))
			case strings.HasPrefix(c, "unknown:"):
				add(fmt.Sprintf("desic C backend: type alias %q targets unknown type %q (define it or use --backend=llvm)",
					v.Name, strings.TrimPrefix(c, "unknown:")))
			}
		}
	}

	return diags
}

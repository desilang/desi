package c

import (
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/check"
)

// validateForCBackend scans the AST for types the C backend cannot handle.
// It returns user-facing diagnostics to be emitted as `#error` lines.
func validateForCBackend(f *ast.File, info *check.Info) []error {
	var diags []error

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

	for _, d := range f.Decls {
		switch v := d.(type) {
		case *ast.FuncDecl:
			switch c := classify(v.Ret); {
			case strings.HasPrefix(c, "unsupported:"):
				typ := strings.TrimPrefix(c, "unsupported:")
				diags = append(diags, CGErrorfT(
					"unsupported_return_type", "DCE0001", "unsupported return type",
					"%s in function %q: return type %q is not supported by the C backend (use i8/i16/i32/i64/isize or u8/u16/u32/u64/usize, bool, str, f32, f64, none; otherwise use --backend=llvm)",
					v.Name, typ,
				))
			case strings.HasPrefix(c, "unknown:"):
				typ := strings.TrimPrefix(c, "unknown:")
				diags = append(diags, CGErrorfT(
					"unknown_return_type", "DCE0002", "unknown return type",
					"%s in function %q: %q (define it as a struct/enum or use --backend=llvm)",
					v.Name, typ,
				))
			case c == "future" && !v.Async:
				diags = append(diags, CGErrorfT(
					"non_async_returns_future", "DCE0003", "non-async function returns future",
					"%s: function %q returns 'future' but is not marked 'async' (mark it 'async' or use --backend=llvm)",
					v.Name,
				))
			}
			for _, p := range v.Params {
				switch c := classify(p.Type); {
				case c == "none":
					diags = append(diags, CGErrorfT(
						"param_none_not_allowed", "DCE0010", "parameter cannot have type 'none'",
						"%s: parameter %q of function %q cannot have type 'none'",
						p.Name, v.Name,
					))
				case c == "future":
					diags = append(diags, CGErrorfT(
						"param_future_not_supported", "DCE0011", "parameter cannot have type 'future' in C backend",
						"%s: parameter %q of function %q cannot have type 'future' (not supported by C backend)",
						p.Name, v.Name,
					))
				case strings.HasPrefix(c, "unsupported:"):
					typ := strings.TrimPrefix(c, "unsupported:")
					diags = append(diags, CGErrorfT(
						"unsupported_param_type", "DCE0012", "unsupported parameter type",
						"%s: %q for %q in function %q (use i8/i16/i32/i64/isize or u8/u16/u32/u64/usize, bool, str, f32, f64; otherwise use --backend=llvm)",
						typ, p.Name, v.Name,
					))
				case strings.HasPrefix(c, "unknown:"):
					typ := strings.TrimPrefix(c, "unknown:")
					diags = append(diags, CGErrorfT(
						"unknown_param_type", "DCE0013", "unknown parameter type",
						"%s: %q for %q in function %q (define it or use --backend=llvm)",
						typ, p.Name, v.Name,
					))
				}
			}

		case *ast.StructDecl:
			for _, ft := range v.Fields {
				switch c := classify(ft.Type); {
				case c == "none":
					diags = append(diags, CGErrorfT(
						"field_none_not_allowed", "DCE0020", "struct field cannot have type 'none'",
						"%s: field %q of struct %q cannot have type 'none'",
						ft.Name, v.Name,
					))
				case c == "future":
					diags = append(diags, CGErrorfT(
						"field_future_not_supported", "DCE0021", "struct field cannot have type 'future'",
						"%s: field %q of struct %q cannot have type 'future' (not supported by C backend)",
						ft.Name, v.Name,
					))
				case strings.HasPrefix(c, "unsupported:"):
					typ := strings.TrimPrefix(c, "unsupported:")
					diags = append(diags, CGErrorfT(
						"unsupported_field_type", "DCE0022", "unsupported struct field type",
						"%s: %q for %q in struct %q (use i8/i16/i32/i64/isize or u8/u16/u32/u64/usize, bool, str, f32, f64; otherwise use --backend=llvm)",
						typ, ft.Name, v.Name,
					))
				case strings.HasPrefix(c, "unknown:"):
					typ := strings.TrimPrefix(c, "unknown:")
					diags = append(diags, CGErrorfT(
						"unknown_field_type", "DCE0023", "unknown struct field type",
						"%s: %q for %q in struct %q (define it or use --backend=llvm)",
						typ, ft.Name, v.Name,
					))
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
					diags = append(diags, CGErrorfT(
						"enum_payload_future_not_supported", "DCE0030", "enum payload cannot be 'future'",
						"%s for %s.%s (not supported by C backend)",
						v.Name, ev.Name,
					))
				case strings.HasPrefix(c, "unsupported:"):
					typ := strings.TrimPrefix(c, "unsupported:")
					diags = append(diags, CGErrorfT(
						"enum_payload_unsupported", "DCE0031", "unsupported enum payload type",
						"%s: %q for %s.%s (use i8/i16/i32/i64/isize or u8/u16/u32/u64/usize, bool, str, f32, f64; otherwise use --backend=llvm)",
						typ, v.Name, ev.Name,
					))
				case strings.HasPrefix(c, "unknown:"):
					typ := strings.TrimPrefix(c, "unknown:")
					diags = append(diags, CGErrorfT(
						"enum_payload_unknown", "DCE0032", "unknown enum payload type",
						"%s: %q for %s.%s (define it or use --backend=llvm)",
						typ, v.Name, ev.Name,
					))
				}
			}

		case *ast.TypeDecl:
			switch c := classify(v.Underlying); {
			case c == "future":
				diags = append(diags, CGErrorfT(
					"type_alias_future_not_supported", "DCE0040", "type alias cannot target 'future'",
					"%s: %q cannot target 'future' (not supported by C backend)",
					v.Name,
				))
			case strings.HasPrefix(c, "unsupported:"):
				typ := strings.TrimPrefix(c, "unsupported:")
				diags = append(diags, CGErrorfT(
					"type_alias_unsupported", "DCE0041", "unsupported type alias target",
					"%s: %q targets unsupported type %q (use i8/i16/i32/i64/isize or u8/u16/u32/u64/usize, bool, str, f32, f64; otherwise use --backend=llvm)",
					v.Name, typ,
				))
			case strings.HasPrefix(c, "unknown:"):
				typ := strings.TrimPrefix(c, "unknown:")
				diags = append(diags, CGErrorfT(
					"type_alias_unknown", "DCE0042", "unknown type alias target",
					"%s: %q targets unknown type %q (define it or use --backend=llvm)",
					v.Name, typ,
				))
			}
		}
	}

	return diags
}

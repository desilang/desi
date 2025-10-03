package c

import (
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/check"
)

// ---- signatures & helpers ----

type sig struct {
	// "void" | scalar kind ("bool","i8","u16","i32","u64","isize","usize","f32","f64","str","future")
	// or "struct:<Name>" | "enum:<Name>"
	ret    string
	params []string
}

func collectFuncSigs(f *ast.File, info *check.Info) map[string]sig {
	m := make(map[string]sig)
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		var s sig

		// Prefer checker info (it knows about async/future), but map textual type for precision.
		if info != nil {
			if si, ok := info.Funcs[fn.Name]; ok && si.Async {
				s.ret = "future"
			} else {
				s.ret = typeToKindOrStruct(fn.Ret, info)
			}
		} else {
			s.ret = typeToKindOrStruct(fn.Ret, info)
		}
		for _, p := range fn.Params {
			s.params = append(s.params, typeToKindOrStruct(p.Type, info))
		}
		m[fn.Name] = s
	}
	return m
}

func findMain(f *ast.File) *ast.FuncDecl {
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name == "main" {
			return fn
		}
	}
	return nil
}

// Map textual types to compact codegen kinds.
// Returns: "void" | scalar kind | "str" | "future" | "struct:<Name>" | "enum:<Name>"
func typeToKindOrStruct(t string, info *check.Info) string {
	lc := strings.ToLower(strings.TrimSpace(t))
	switch lc {
	case "", "none", "void":
		return "void"
	case "bool":
		return "bool"
	case "i8", "i16", "i32", "i64", "isize",
		"u8", "u16", "u32", "u64", "usize",
		"f32", "f64",
		"int": // int ≡ i32
		return normalizeScalarKind(lc)
	case "str", "string":
		return "str"
	case "future":
		return "future"
	default:
		// user types (struct/enum) if known
		raw := strings.TrimSpace(t)
		if info != nil {
			if _, ok := info.Structs[raw]; ok {
				return "struct:" + raw
			}
			if info.Enums != nil {
				if _, ok := info.Enums[raw]; ok {
					return "enum:" + raw
				}
			}
		}
		// Unknown user types default to int at C level, but keep it simple here:
		return "i32"
	}
}

// Collapse aliases to canonical scalar tags used by cType().
func normalizeScalarKind(k string) string {
	switch k {
	case "int":
		return "i32"
	case "string":
		return "str"
	default:
		return k
	}
}

// Map compact kind → C type used in signatures.
func cType(kind string) string {
	switch kind {
	case "void":
		return "void"
	case "str":
		return "const char*"
	case "future":
		return "struct desi_future"

	// bool: use int for ABI simplicity (you can switch to _Bool if desired)
	case "bool":
		return "int"

	// Signed integers
	case "i8":
		return "int8_t"
	case "i16":
		return "int16_t"
	case "i32":
		return "int32_t"
	case "i64":
		return "int64_t"
	case "isize":
		return "intptr_t"

	// Unsigned integers
	case "u8":
		return "uint8_t"
	case "u16":
		return "uint16_t"
	case "u32":
		return "uint32_t"
	case "u64":
		return "uint64_t"
	case "usize":
		return "uintptr_t"

	// Floats
	case "f32":
		return "float"
	case "f64":
		return "double"
	}

	// struct/enum:<Name>
	if strings.HasPrefix(kind, "struct:") || strings.HasPrefix(kind, "enum:") {
		return kind[strings.Index(kind, ":")+1:]
	}

	// Fallback
	return "int32_t"
}

// Direct map from textual type → C type (used for params list building).
func cTypeFromText(t string, info *check.Info) string {
	k := typeToKindOrStruct(t, info)
	return cType(k)
}

func cParamList(fn *ast.FuncDecl, info *check.Info) string {
	var parts []string
	for _, p := range fn.Params {
		parts = append(parts, cTypeFromText(p.Type, info)+" "+p.Name)
	}
	return strings.Join(parts, ", ")
}

func isStructType(text string, info *check.Info) bool {
	if info == nil || info.Structs == nil {
		return false
	}
	tt := strings.TrimSpace(text)
	_, ok := info.Structs[tt]
	return ok
}

func isStrText(text string) bool {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "str", "string":
		return true
	}
	return false
}

func isEnumType(text string, info *check.Info) bool {
	if info == nil || info.Enums == nil {
		return false
	}
	tt := strings.TrimSpace(text)
	_, ok := info.Enums[tt]
	return ok
}

func isNoneText(text string) bool {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "", "none":
		return true
	}
	return false
}

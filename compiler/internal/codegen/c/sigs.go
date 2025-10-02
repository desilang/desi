package c

import (
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/check"
)

// ---- signatures & helpers ----

type sig struct {
	// "void" | "int" | "str" | "struct:<Name>" | "enum:<Name>" | "future"
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
		// Prefer checker info (it knows about async/future).
		if info != nil {
			if si, ok := info.Funcs[fn.Name]; ok {
				if si.Async {
					s.ret = "future"
				} else {
					s.ret = typeToKindOrStruct(fn.Ret, info)
				}
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

// map textual types to compact codegen kind
// "void" | "int" | "str" | "struct:<Name>" | "enum:<Name>"
func typeToKindOrStruct(t string, info *check.Info) string {
	tt := strings.TrimSpace(strings.ToLower(t))
	switch tt {
	case "", "none", "void":
		return "void"
	case "i32", "int", "u32", "bool":
		return "int"
	case "str", "string":
		return "str"
	case "future":
		return "future"
	default:
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
		return "int"
	}
}

func cType(kind string) string {
	switch kind {
	case "void":
		return "void"
	case "str":
		return "const char*"
	case "int":
		return "int"
	case "future":
		return "struct desi_future"
	default:
		if strings.HasPrefix(kind, "struct:") || strings.HasPrefix(kind, "enum:") {
			return kind[strings.Index(kind, ":")+1:] // drop "struct:" or "enum:"
		}
		return "int"
	}
}

func cTypeFromText(t string, info *check.Info) string {
	tt := strings.TrimSpace(t)
	switch strings.ToLower(tt) {
	case "", "none", "void":
		return "void"
	case "i32", "int", "u32", "bool":
		return "int"
	case "str", "string":
		return "const char*"
	case "future":
		return "struct desi_future"
	}
	if info != nil {
		if _, ok := info.Structs[tt]; ok {
			return tt
		}
		if info.Enums != nil {
			if _, ok := info.Enums[tt]; ok {
				return tt
			}
		}
	}
	return "int"
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

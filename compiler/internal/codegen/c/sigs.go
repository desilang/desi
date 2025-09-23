package c

import (
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/check"
)

// ---- signatures & helpers ----

type sig struct {
	ret    string // "int"|"str"|"void"
	params []string
}

func collectFuncSigs(f *ast.File) map[string]sig {
	m := make(map[string]sig)
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		s := sig{ret: typeToKind(fn.Ret)}
		for _, p := range fn.Params {
			s.params = append(s.params, typeToKind(p.Type))
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

func typeToKind(t string) string {
	t = strings.TrimSpace(strings.ToLower(t))
	switch t {
	case "", "void":
		return "void"
	case "i32", "int", "u32", "bool":
		return "int"
	case "str", "string":
		return "str"
	default:
		// Unknown textual types (including struct names) — treat as int for returns (stage-1).
		return "int"
	}
}

func cType(kind string) string {
	switch kind {
	case "void":
		return "void"
	case "str":
		return "const char*"
	default:
		return "int"
	}
}

func cTypeFromText(t string, info *check.Info) string {
	tt := strings.TrimSpace(t)
	switch strings.ToLower(tt) {
	case "", "void":
		return "void"
	case "i32", "int", "u32", "bool":
		return "int"
	case "str", "string":
		return "const char*"
	}
	// struct?
	if info != nil && info.Structs != nil {
		if _, ok := info.Structs[tt]; ok {
			return tt
		}
	}
	// fallback
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

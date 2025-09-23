package c

import (
	"bytes"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/check"
	"github.com/desilang/desi/compiler/internal/term"
)

// ---- struct typedef emission ----

func emitStructTypedef(b *bytes.Buffer, sd *ast.StructDecl, info *check.Info) {
	term.Wprintf(b, "typedef struct {\n")
	for _, fld := range sd.Fields {
		ct := cFieldType(fld.Type, info)
		term.Wprintf(b, "  %s %s;\n", ct, fld.Name)
	}
	term.Wprintf(b, "} %s;", sd.Name)
}

// Map a textual field type into a C type string.
func cFieldType(t string, info *check.Info) string {
	tt := strings.TrimSpace(t)
	switch strings.ToLower(tt) {
	case "", "void":
		return "int"
	case "i32", "int", "u32", "bool":
		return "int"
	case "str", "string":
		return "const char*"
	}
	// Struct name?
	if info != nil && info.Structs != nil {
		if _, ok := info.Structs[tt]; ok {
			return tt // use typedef name directly
		}
	}
	return "int"
}

// fieldTypeOf looks up the textual type of a field on a struct name in Info.Structs.
// Works with Info.Structs value type check.StructInfo { Fields map[string]string }.
func fieldTypeOf(info *check.Info, structName, fieldName string) (string, bool) {
	if info == nil || info.Structs == nil {
		return "", false
	}
	si, ok := info.Structs[strings.TrimSpace(structName)]
	if !ok {
		return "", false
	}
	if si.Fields == nil {
		return "", false
	}
	ft, ok := si.Fields[fieldName]
	return ft, ok
}

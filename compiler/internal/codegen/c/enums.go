package c

import (
	"bytes"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/check"
	"github.com/desilang/desi/compiler/internal/term"
)

func emitEnumTypedef(b *bytes.Buffer, ed *ast.EnumDecl, info *check.Info) {
	// 1) Tag constants
	for i, v := range ed.Variants {
		term.Wprintf(b, "#define %s_%s %d\n", ed.Name, v.Name, i)
	}
	term.Wprintf(b, "\n")

	// 2) Struct wrapper (tag + optional union payload)
	hasPayload := false
	for _, v := range ed.Variants {
		if !isNoneText(v.Payload) {
			hasPayload = true
			break
		}
	}

	term.Wprintf(b, "typedef struct {\n")
	term.Wprintf(b, "  int tag;\n")
	if hasPayload {
		term.Wprintf(b, "  union {\n")
		for _, v := range ed.Variants {
			if isNoneText(v.Payload) {
				continue
			}
			term.Wprintf(b, "    %s %s;\n", cTypeFromText(v.Payload, info), v.Name)
		}
		term.Wprintf(b, "  } as;\n")
	}
	term.Wprintf(b, "} %s;\n", ed.Name)
}

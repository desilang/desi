package c

import (
	"bytes"
	"strconv"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/check"
	"github.com/desilang/desi/compiler/internal/term"
)

type env struct {
	fn          *ast.FuncDecl
	info        *check.Info
	sigs        map[string]sig
	vars        map[string]string // name -> textual type ("int"/"str"/struct name/...)
	retKind     string            // "void"|"int"|"str"|"struct:<Name>"
	defers      []ast.Expr        // function-scope defers (LIFO)
	tempCounter int
}

func (e *env) newTemp() string {
	e.tempCounter++
	return "__tmp" + strconv.Itoa(e.tempCounter)
}

func emitFunc(b *bytes.Buffer, fn *ast.FuncDecl, sigs map[string]sig, info *check.Info, isMain bool) {
	e := &env{
		fn:      fn,
		info:    info,
		sigs:    sigs,
		vars:    map[string]string{},
		retKind: typeToKindOrStruct(fn.Ret, info),
		defers:  nil,
	}
	for _, p := range fn.Params {
		// keep textual type so we can know struct names
		if strings.TrimSpace(p.Type) == "" {
			e.vars[p.Name] = "int"
		} else {
			e.vars[p.Name] = strings.TrimSpace(p.Type)
		}
	}

	// signature
	if isMain {
		term.Wprintf(b, "int main(void) {\n")
	} else {
		term.Wprintf(b, "static %s %s(%s) {\n",
			cType(e.retKind), fn.Name, cParamList(fn, info))
	}

	// body
	for _, s := range fn.Body {
		emitStmt(b, 2, s, e)
	}

	// On normal fallthrough, run defers then synthesize default return if needed.
	if len(e.defers) > 0 {
		emitDefers(b, 2, e)
	}
	if !hasTailReturn(fn.Body) {
		switch e.retKind {
		case "void":
			// no-op
		case "int":
			term.Wprintf(b, "  return 0;\n")
		case "str":
			term.Wprintf(b, "  return \"\";\n")
		default:
			if strings.HasPrefix(e.retKind, "struct:") {
				name := strings.TrimPrefix(e.retKind, "struct:")
				// Zero-init struct return on fallthrough.
				term.Wprintf(b, "  return (%s){0};\n", name)
			} else {
				term.Wprintf(b, "  return 0;\n")
			}
		}
	}
	term.Wprintf(b, "}\n")
}

func hasTailReturn(body []ast.Stmt) bool {
	if len(body) == 0 {
		return false
	}
	_, ok := body[len(body)-1].(*ast.ReturnStmt)
	return ok
}

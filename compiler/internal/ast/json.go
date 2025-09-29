package ast

import (
	"encoding/json"
)

/*
Stable JSON for AST comparison/parity tests.

We convert the Go AST (with interfaces) into a plain, tagged JSON tree.
Every node has a "kind" field; spans are included as {start{line,col}, end{line,col}}.
*/

type jPos struct {
	Line int `json:"line"`
	Col  int `json:"col"`
}
type jSpan struct {
	Start jPos `json:"start"`
	End   jPos `json:"end"`
}

/* ---------- top level ---------- */

type jFile struct {
	Kind        string        `json:"kind"` // "File"
	Package     *jPackage     `json:"package,omitempty"`
	Imports     []jImport     `json:"imports,omitempty"`
	FromImports []jFromImport `json:"from_imports,omitempty"`
	Decls       []any         `json:"decls,omitempty"`
}

type jPackage struct {
	Kind string `json:"kind"` // "PackageDecl"
	Name string `json:"name"`
}

type jImport struct {
	Kind string `json:"kind"` // "ImportDecl"
	Path string `json:"path"`
	As   string `json:"as,omitempty"`
	Span jSpan  `json:"span"`
}

/*** NEW: from-imports ***/

type jFromImport struct {
	Kind   string        `json:"kind"` // "FromImportDecl"
	Module string        `json:"module"`
	Items  []jImportItem `json:"items,omitempty"`
	Span   jSpan         `json:"span"`
}

type jImportItem struct {
	Kind string `json:"kind"` // "ImportItem"
	Name string `json:"name"`
	As   string `json:"as,omitempty"`
	Span jSpan  `json:"span"`
}

/* ---------- decls ---------- */

type jFuncDecl struct {
	Kind   string   `json:"kind"` // "FuncDecl"
	Name   string   `json:"name"`
	Params []jParam `json:"params,omitempty"`
	Ret    string   `json:"ret,omitempty"`
	Body   []any    `json:"body,omitempty"`
	Pub    bool     `json:"pub,omitempty"` // NEW (M10)
	Span   jSpan    `json:"span"`
}

type jTypeDecl struct {
	Kind       string `json:"kind"` // "TypeDecl"
	Name       string `json:"name"`
	Underlying string `json:"underlying"`
	Span       jSpan  `json:"span"`
}

type jStructDecl struct {
	Kind   string   `json:"kind"` // "StructDecl"
	Name   string   `json:"name"`
	Fields []jField `json:"fields"`
	Pub    bool     `json:"pub,omitempty"` // NEW (M10)
	Span   jSpan    `json:"span"`
}

type jParam struct {
	Kind string `json:"kind"` // "Param"
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
	Span jSpan  `json:"span"`
}

type jField struct {
	Kind string `json:"kind"` // "Field"
	Name string `json:"name"`
	Type string `json:"type"`
	Span jSpan  `json:"span"`
}

/* ---------- stmts ---------- */

type jLetBind struct {
	Kind string `json:"kind"` // "LetBind"
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
	Span jSpan  `json:"span"`
}

type jLetStmt struct {
	Kind      string     `json:"kind"` // "LetStmt"
	Mutable   bool       `json:"mutable"`
	Binds     []jLetBind `json:"binds"`
	GroupType string     `json:"groupType,omitempty"`
	Values    []any      `json:"values"`
	Span      jSpan      `json:"span"`
}

type jAssignStmt struct {
	Kind  string   `json:"kind"` // "AssignStmt"
	Names []string `json:"names"`
	Exprs []any    `json:"exprs"`
	Span  jSpan    `json:"span"`
}

type jReturnStmt struct {
	Kind string `json:"kind"` // "ReturnStmt"
	Expr any    `json:"expr,omitempty"`
	Span jSpan  `json:"span"`
}

type jExprStmt struct {
	Kind string `json:"kind"` // "ExprStmt"
	Expr any    `json:"expr"`
	Span jSpan  `json:"span"`
}

type jIfStmt struct {
	Kind  string    `json:"kind"` // "IfStmt"
	Cond  any       `json:"cond"`
	Then  []any     `json:"then"`
	Elifs []jElseIf `json:"elifs,omitempty"`
	Else  []any     `json:"else,omitempty"`
	Span  jSpan     `json:"span"`
}

type jElseIf struct {
	Kind string `json:"kind"` // "ElseIf"
	Cond any    `json:"cond"`
	Body []any  `json:"body"`
	Span jSpan  `json:"span"`
}

type jWhileStmt struct {
	Kind string `json:"kind"` // "WhileStmt"
	Cond any    `json:"cond"`
	Body []any  `json:"body"`
	Span jSpan  `json:"span"`
}

type jDeferStmt struct {
	Kind string `json:"kind"` // "DeferStmt"
	Call any    `json:"call"`
	Span jSpan  `json:"span"`
}

/* ---------- exprs ---------- */

type jIdentExpr struct {
	Kind string `json:"kind"` // "IdentExpr"
	Name string `json:"name"`
	Span jSpan  `json:"span"`
}

type jIntLit struct {
	Kind  string `json:"kind"` // "IntLit"
	Value string `json:"value"`
	Span  jSpan  `json:"span"`
}

type jStrLit struct {
	Kind  string `json:"kind"` // "StrLit"
	Value string `json:"value"`
	Span  jSpan  `json:"span"`
}

type jBoolLit struct {
	Kind  string `json:"kind"` // "BoolLit"
	Value bool   `json:"value"`
	Span  jSpan  `json:"span"`
}

type jCallExpr struct {
	Kind   string `json:"kind"` // "CallExpr"
	Callee any    `json:"callee"`
	Args   []any  `json:"args"`
	Span   jSpan  `json:"span"`
}

type jIndexExpr struct {
	Kind  string `json:"kind"` // "IndexExpr"
	Seq   any    `json:"seq"`
	Index any    `json:"index"`
	Span  jSpan  `json:"span"`
}

type jFieldExpr struct {
	Kind string `json:"kind"` // "FieldExpr"
	X    any    `json:"x"`
	Name string `json:"name"`
	Span jSpan  `json:"span"`
}

type jUnaryExpr struct {
	Kind string `json:"kind"` // "UnaryExpr"
	Op   string `json:"op"`
	X    any    `json:"x"`
	Span jSpan  `json:"span"`
}

type jBinaryExpr struct {
	Kind  string `json:"kind"` // "BinaryExpr"
	Op    string `json:"op"`
	Left  any    `json:"left"`
	Right any    `json:"right"`
	Span  jSpan  `json:"span"`
}

/* ---------- marshal entrypoint ---------- */

func MarshalFileJSON(f *File) ([]byte, error) {
	out := toJFile(f)
	return json.MarshalIndent(out, "", "  ")
}

/* ---------- converters ---------- */

func spanJS(s Span) jSpan {
	return jSpan{Start: jPos{s.Start.Line, s.Start.Col}, End: jPos{s.End.Line, s.End.Col}}
}

func toJFile(f *File) jFile {
	var pkg *jPackage
	if f.Pkg != nil {
		pkg = &jPackage{Kind: "PackageDecl", Name: f.Pkg.Name}
	}
	imps := make([]jImport, 0, len(f.Imports))
	for _, im := range f.Imports {
		imps = append(imps, jImport{
			Kind: "ImportDecl",
			Path: im.Path,
			As:   im.As,
			Span: spanJS(im.Span),
		})
	}
	fimps := make([]jFromImport, 0, len(f.FromImports))
	for _, fi := range f.FromImports {
		items := make([]jImportItem, 0, len(fi.Items))
		for _, it := range fi.Items {
			items = append(items, jImportItem{
				Kind: "ImportItem",
				Name: it.Name,
				As:   it.As,
				Span: spanJS(it.Span),
			})
		}
		fimps = append(fimps, jFromImport{
			Kind:   "FromImportDecl",
			Module: fi.Module,
			Items:  items,
			Span:   spanJS(fi.Span),
		})
	}
	d := make([]any, 0, len(f.Decls))
	for _, dec := range f.Decls {
		switch v := dec.(type) {
		case *FuncDecl:
			d = append(d, toJFunc(v))
		case *TypeDecl:
			d = append(d, toJType(v))
		case *StructDecl:
			d = append(d, toJStruct(v))
		default:
			// future decl kinds
		}
	}
	return jFile{Kind: "File", Package: pkg, Imports: imps, FromImports: fimps, Decls: d}
}

func toJFunc(fd *FuncDecl) jFuncDecl {
	ps := make([]jParam, 0, len(fd.Params))
	for _, p := range fd.Params {
		ps = append(ps, jParam{
			Kind: "Param",
			Name: p.Name,
			Type: p.Type,
			Span: spanJS(p.Span),
		})
	}
	body := make([]any, 0, len(fd.Body))
	for _, st := range fd.Body {
		body = append(body, toJStmt(st))
	}
	return jFuncDecl{
		Kind:   "FuncDecl",
		Name:   fd.Name,
		Params: ps,
		Ret:    fd.Ret,
		Body:   body,
		Pub:    fd.Pub, // NEW
		Span:   spanJS(fd.Span),
	}
}

func toJType(td *TypeDecl) jTypeDecl {
	return jTypeDecl{
		Kind:       "TypeDecl",
		Name:       td.Name,
		Underlying: td.Underlying,
		Span:       spanJS(td.Span),
	}
}

func toJStruct(sd *StructDecl) jStructDecl {
	fs := make([]jField, 0, len(sd.Fields))
	for _, f := range sd.Fields {
		fs = append(fs, jField{
			Kind: "Field",
			Name: f.Name,
			Type: f.Type,
			Span: spanJS(f.Span),
		})
	}
	return jStructDecl{
		Kind:   "StructDecl",
		Name:   sd.Name,
		Fields: fs,
		Pub:    sd.Pub, // NEW
		Span:   spanJS(sd.Span),
	}
}

func toJStmt(s Stmt) any {
	switch v := s.(type) {
	case *LetStmt:
		bs := make([]jLetBind, 0, len(v.Binds))
		for _, b := range v.Binds {
			bs = append(bs, jLetBind{Kind: "LetBind", Name: b.Name, Type: b.Type, Span: spanJS(b.Span)})
		}
		vals := make([]any, 0, len(v.Values))
		for _, e := range v.Values {
			vals = append(vals, toJExpr(e))
		}
		return jLetStmt{Kind: "LetStmt", Mutable: v.Mutable, Binds: bs, GroupType: v.GroupType, Values: vals, Span: spanJS(v.Span)}
	case *AssignStmt:
		es := make([]any, 0, len(v.Exprs))
		for _, e := range v.Exprs {
			es = append(es, toJExpr(e))
		}
		return jAssignStmt{Kind: "AssignStmt", Names: append([]string{}, v.Names...), Exprs: es, Span: spanJS(v.Span)}
	case *ReturnStmt:
		var e any
		if v.Expr != nil {
			e = toJExpr(v.Expr)
		}
		return jReturnStmt{Kind: "ReturnStmt", Expr: e, Span: spanJS(v.Span)}
	case *ExprStmt:
		return jExprStmt{Kind: "ExprStmt", Expr: toJExpr(v.Expr), Span: spanJS(v.Span)}
	case *IfStmt:
		thenStmts := make([]any, 0, len(v.Then))
		for _, s2 := range v.Then {
			thenStmts = append(thenStmts, toJStmt(s2))
		}
		elifs := make([]jElseIf, 0, len(v.Elifs))
		for _, el := range v.Elifs {
			body := make([]any, 0, len(el.Body))
			for _, s2 := range el.Body {
				body = append(body, toJStmt(s2))
			}
			elifs = append(elifs, jElseIf{
				Kind: "ElseIf", Cond: toJExpr(el.Cond), Body: body, Span: spanJS(el.Span),
			})
		}
		var els []any
		if len(v.Else) > 0 {
			els = make([]any, 0, len(v.Else))
			for _, s2 := range v.Else {
				els = append(els, toJStmt(s2))
			}
		}
		return jIfStmt{
			Kind: "IfStmt", Cond: toJExpr(v.Cond), Then: thenStmts, Elifs: elifs, Else: els, Span: spanJS(v.Span),
		}
	case *WhileStmt:
		body := make([]any, 0, len(v.Body))
		for _, s2 := range v.Body {
			body = append(body, toJStmt(s2))
		}
		return jWhileStmt{Kind: "WhileStmt", Cond: toJExpr(v.Cond), Body: body, Span: spanJS(v.Span)}
	case *DeferStmt:
		return jDeferStmt{Kind: "DeferStmt", Call: toJExpr(v.Call), Span: spanJS(v.Span)}
	default:
		return map[string]any{"kind": "UnknownStmt"}
	}
}

func toJExpr(e Expr) any {
	switch v := e.(type) {
	case *IdentExpr:
		return jIdentExpr{Kind: "IdentExpr", Name: v.Name, Span: spanJS(v.Span)}
	case *IntLit:
		return jIntLit{Kind: "IntLit", Value: v.Value, Span: spanJS(v.Span)}
	case *StrLit:
		return jStrLit{Kind: "StrLit", Value: v.Value, Span: spanJS(v.Span)}
	case *BoolLit:
		return jBoolLit{Kind: "BoolLit", Value: v.Value, Span: spanJS(v.Span)}
	case *CallExpr:
		args := make([]any, 0, len(v.Args))
		for _, a := range v.Args {
			args = append(args, toJExpr(a))
		}
		return jCallExpr{Kind: "CallExpr", Callee: toJExpr(v.Callee), Args: args, Span: spanJS(v.Span)}
	case *IndexExpr:
		return jIndexExpr{Kind: "IndexExpr", Seq: toJExpr(v.Seq), Index: toJExpr(v.Index), Span: spanJS(v.Span)}
	case *FieldExpr:
		return jFieldExpr{Kind: "FieldExpr", X: toJExpr(v.X), Name: v.Name, Span: spanJS(v.Span)}
	case *UnaryExpr:
		return jUnaryExpr{Kind: "UnaryExpr", Op: v.Op, X: toJExpr(v.X), Span: spanJS(v.Span)}
	case *BinaryExpr:
		return jBinaryExpr{Kind: "BinaryExpr", Op: v.Op, Left: toJExpr(v.Left), Right: toJExpr(v.Right), Span: spanJS(v.Span)}
	default:
		return map[string]any{"kind": "UnknownExpr"}
	}
}

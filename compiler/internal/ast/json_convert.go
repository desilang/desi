package ast

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
		case *EnumDecl:
			d = append(d, toJEnum(v))
		case *ConstDecl:
			d = append(d, toJConst(v))
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
		Pub:    fd.Pub,   // NEW
		Async:  fd.Async, // NEW (M11)
		Span:   spanJS(fd.Span),
	}
}

func toJType(td *TypeDecl) jTypeDecl {
	return jTypeDecl{
		Kind:       "TypeDecl",
		Name:       td.Name,
		Underlying: td.Underlying,
		Pub:        td.Pub, // NEW
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

func toJEnum(ed *EnumDecl) jEnumDecl {
	vs := make([]jEnumVariant, 0, len(ed.Variants))
	for _, v := range ed.Variants {
		vs = append(vs, jEnumVariant{
			Kind:    "EnumVariant",
			Name:    v.Name,
			Payload: v.Payload,
			Span:    spanJS(v.Span),
		})
	}
	return jEnumDecl{
		Kind:     "EnumDecl",
		Name:     ed.Name,
		Variants: vs,
		Pub:      ed.Pub, // NEW
		Span:     spanJS(ed.Span),
	}
}

func toJConst(cd *ConstDecl) jConstDecl {
	return jConstDecl{
		Kind:    "ConstDecl",
		Name:    cd.Name,
		Type:    cd.Type,
		Value:   toJExpr(cd.Value),
		Pub:     cd.Pub,
		Mutable: cd.Mutable,
		Span:    spanJS(cd.Span),
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
	case *AwaitExpr:
		return jAwaitExpr{Kind: "AwaitExpr", Expr: toJExpr(v.Expr), Span: spanJS(v.Span)}
	case *FloatLit:
		return jFloatLit{Kind: "FloatLit", Value: v.Value, Span: spanJS(v.Span)}
	case *StructLit:
		fs := make([]jStructLitField, 0, len(v.Fields))
		for _, f := range v.Fields {
			fs = append(fs, jStructLitField{
				Kind:  "StructLitField",
				Name:  f.Name,
				Value: toJExpr(f.Value),
				Span:  spanJS(f.Span),
			})
		}
		return jStructLit{
			Kind:   "StructLit",
			Name:   v.Name,
			Fields: fs,
			Span:   spanJS(v.Span),
		}
	default:
		return map[string]any{"kind": "UnknownExpr"}
	}
}

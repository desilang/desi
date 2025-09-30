package ast

import (
	"fmt"
)

/* ---------- decls ---------- */

func fromJDecl(v any) (Decl, error) {
	m, ok := asMap(v)
	if !ok {
		return nil, fmt.Errorf("AST JSON: decl not object")
	}
	switch getString(m, "kind") {
	case "FuncDecl":
		return fromJFunc(m)
	case "TypeDecl":
		return fromJType(m)
	case "StructDecl":
		return fromJStruct(m)
	case "EnumDecl":
		return fromJEnum(m)
	case "ConstDecl":
		return fromJConst(m)
	default:
		// ignore unknown decl kinds for now
		return nil, nil
	}
}

func fromJFunc(m map[string]any) (*FuncDecl, error) {
	fd := &FuncDecl{
		Name: getString(m, "name"),
		Ret:  getString(m, "ret"),
		Pub:  getBool(m, "pub"),
		Span: parseSpan(getMap(m, "span")),
	}
	// params
	if arr := getSlice(m, "params"); arr != nil {
		for _, p := range arr {
			pm, ok := asMap(p)
			if !ok || getString(pm, "kind") != "Param" {
				continue
			}
			fd.Params = append(fd.Params, Param{
				Name: getString(pm, "name"),
				Type: getString(pm, "type"),
				Span: parseSpan(getMap(pm, "span")),
			})
		}
	}
	// body
	if arr := getSlice(m, "body"); arr != nil {
		for _, s := range arr {
			st, err := fromJStmt(s)
			if err != nil {
				return nil, err
			}
			if st != nil {
				fd.Body = append(fd.Body, st)
			}
		}
	}
	return fd, nil
}

func fromJType(m map[string]any) (*TypeDecl, error) {
	return &TypeDecl{
		Name:       getString(m, "name"),
		Underlying: getString(m, "underlying"),
		Pub:        getBool(m, "pub"),
		Span:       parseSpan(getMap(m, "span")),
	}, nil
}

func fromJStruct(m map[string]any) (*StructDecl, error) {
	sd := &StructDecl{
		Name: getString(m, "name"),
		Pub:  getBool(m, "pub"),
		Span: parseSpan(getMap(m, "span")),
	}
	if arr := getSlice(m, "fields"); arr != nil {
		for _, f := range arr {
			fm, ok := asMap(f)
			if !ok || getString(fm, "kind") != "Field" {
				continue
			}
			sd.Fields = append(sd.Fields, Field{
				Name: getString(fm, "name"),
				Type: getString(fm, "type"),
				Span: parseSpan(getMap(fm, "span")),
			})
		}
	}
	return sd, nil
}

func fromJEnum(m map[string]any) (*EnumDecl, error) {
	ed := &EnumDecl{
		Name: getString(m, "name"),
		Pub:  getBool(m, "pub"),
		Span: parseSpan(getMap(m, "span")),
	}
	if arr := getSlice(m, "variants"); arr != nil {
		for _, v := range arr {
			vm, ok := asMap(v)
			if !ok || getString(vm, "kind") != "EnumVariant" {
				continue
			}
			ed.Variants = append(ed.Variants, EnumVariant{
				Name:    getString(vm, "name"),
				Payload: getString(vm, "payload"),
				Span:    parseSpan(getMap(vm, "span")),
			})
		}
	}
	return ed, nil
}

func fromJConst(m map[string]any) (*ConstDecl, error) {
	v, err := fromJExpr(m["value"])
	if err != nil {
		return nil, err
	}
	return &ConstDecl{
		Name:    getString(m, "name"),
		Type:    getString(m, "type"),
		Value:   v,
		Pub:     getBool(m, "pub"),
		Mutable: getBool(m, "mutable"),
		Span:    parseSpan(getMap(m, "span")),
	}, nil
}

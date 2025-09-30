package ast

import (
	"encoding/json"
	"errors"
)

/* ---------- entrypoint ---------- */

// UnmarshalFileJSON reconstructs a *File from JSON produced by MarshalFileJSON.
func UnmarshalFileJSON(data []byte) (*File, error) {
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	m, ok := asMap(root)
	if !ok || getString(m, "kind") != "File" {
		return nil, errors.New("AST JSON: root is not kind=File")
	}
	var f File

	// package
	if pm := getMap(m, "package"); pm != nil && getString(pm, "kind") == "PackageDecl" {
		f.Pkg = &PackageDecl{Name: getString(pm, "name")}
	}

	// imports
	if arr := getSlice(m, "imports"); arr != nil {
		for _, it := range arr {
			im, ok := asMap(it)
			if !ok || getString(im, "kind") != "ImportDecl" {
				continue
			}
			f.Imports = append(f.Imports, ImportDecl{
				Path: getString(im, "path"),
				As:   getString(im, "as"),
				Span: parseSpan(getMap(im, "span")),
			})
		}
	}

	// from_imports (optional)
	if arr := getSlice(m, "from_imports"); arr != nil {
		for _, it := range arr {
			fm, ok := asMap(it)
			if !ok || getString(fm, "kind") != "FromImportDecl" {
				continue
			}
			fi := FromImportDecl{
				Module: getString(fm, "module"),
				Span:   parseSpan(getMap(fm, "span")),
			}
			if items := getSlice(fm, "items"); items != nil {
				for _, iv := range items {
					im, ok := asMap(iv)
					if !ok || getString(im, "kind") != "ImportItem" {
						continue
					}
					fi.Items = append(fi.Items, ImportItem{
						Name: getString(im, "name"),
						As:   getString(im, "as"),
						Span: parseSpan(getMap(im, "span")),
					})
				}
			}
			f.FromImports = append(f.FromImports, fi)
		}
	}

	// decls
	if arr := getSlice(m, "decls"); arr != nil {
		for _, d := range arr {
			dec, err := fromJDecl(d)
			if err != nil {
				return nil, err
			}
			if dec != nil {
				f.Decls = append(f.Decls, dec)
			}
		}
	}
	return &f, nil
}

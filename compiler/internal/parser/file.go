package parser

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/lexer"
)

func (p *Parser) ParseFile() (*ast.File, error) {
	f := &ast.File{}
	p.skipNewlines()

	// package (optional)
	if p.accept(lexer.TokPackage) {
		name, err := p.parseDottedIdent()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.TokNewline); err != nil {
			return nil, err
		}
		f.Pkg = &ast.PackageDecl{Name: name}
		p.skipNewlines()
	}

	// imports (plain and from-imports)
	for p.at(lexer.TokImport) || p.at(lexer.TokFrom) {
		if p.at(lexer.TokImport) {
			importTok := p.tok
			p.next() // consume 'import'

			path, err := p.parseDottedIdent()
			if err != nil {
				return nil, err
			}

			// Optional: plain module aliasing — `import foo.bar as baz`
			as := ""
			if p.accept(lexer.TokAs) {
				aliasTok, err := p.expectIdent("module alias")
				if err != nil {
					return nil, err
				}
				as = aliasTok.Lex
			}

			nlTok, err := p.expect(lexer.TokNewline)
			if err != nil {
				return nil, err
			}
			f.Imports = append(f.Imports, ast.ImportDecl{
				Path: path,
				As:   as,
				Span: spanTok(importTok, nlTok),
			})
		} else {
			// from-import
			fromTok := p.tok
			p.next() // consume 'from'
			fi, err := p.parseFromImportAt(fromTok)
			if err != nil {
				return nil, err
			}
			f.FromImports = append(f.FromImports, *fi)
		}
		p.skipNewlines()
	}

	// decls
	for !p.at(lexer.TokEOF) {
		// Optional 'pub' prefix before def/struct/let/type/enum
		if p.accept(lexer.TokPub) {
			switch {
			case p.at(lexer.TokAsync):
				// pub async def ...
				if !p.features.Async {
					return nil, ErrAsyncOnlyBeforeDef(p.tok)
				}
				p.next() // consume 'async'
				if !p.accept(lexer.TokDef) {
					return nil, ErrAsyncOnlyBeforeDef(p.tok)
				}
				fn, err := p.parseFuncDecl()
				if err != nil {
					return nil, err
				}
				fn.Pub = true
				fn.Async = true
				f.Decls = append(f.Decls, fn)

			case p.at(lexer.TokDef):
				p.next() // consume 'def'
				fn, err := p.parseFuncDecl()
				if err != nil {
					return nil, err
				}
				fn.Pub = true
				f.Decls = append(f.Decls, fn)

			case p.at(lexer.TokStruct):
				structTok := p.tok
				p.next() // consume 'struct'
				sd, err := p.parseStructDeclAt(structTok)
				if err != nil {
					return nil, err
				}
				sd.Pub = true
				f.Decls = append(f.Decls, sd)

			case p.at(lexer.TokLet):
				letTok := p.tok
				p.next() // consume 'let'
				cd, err := p.parseTopConstAt(letTok, true /*pub*/)
				if err != nil {
					return nil, err
				}
				f.Decls = append(f.Decls, cd)

			case p.at(lexer.TokType):
				typeTok := p.tok
				p.next() // consume 'type'
				td, err := p.parseTypeDeclAt(typeTok)
				if err != nil {
					return nil, err
				}
				td.Pub = true
				f.Decls = append(f.Decls, td)

			case p.at(lexer.TokEnum):
				enumTok := p.tok
				p.next() // consume 'enum'
				ed, err := p.parseEnumDeclAt(enumTok)
				if err != nil {
					return nil, err
				}
				ed.Pub = true
				f.Decls = append(f.Decls, ed)

			default:
				// 'pub' not followed by a known decl keyword
				return nil, ErrAfterPubUnexpected(p.tok)
			}
			p.skipNewlines()
			continue
		}

		switch {
		case p.at(lexer.TokAsync):
			// async def ...
			if !p.features.Async {
				return nil, ErrAsyncOnlyBeforeDef(p.tok)
			}
			p.next() // consume 'async'
			if !p.accept(lexer.TokDef) {
				return nil, ErrAsyncOnlyBeforeDef(p.tok)
			}
			fn, err := p.parseFuncDecl()
			if err != nil {
				return nil, err
			}
			fn.Async = true
			f.Decls = append(f.Decls, fn)

		case p.accept(lexer.TokDef):
			fn, err := p.parseFuncDecl()
			if err != nil {
				return nil, err
			}
			f.Decls = append(f.Decls, fn)

		case p.at(lexer.TokType):
			typeTok := p.tok
			p.next() // consume 'type'
			td, err := p.parseTypeDeclAt(typeTok)
			if err != nil {
				return nil, err
			}
			f.Decls = append(f.Decls, td)

		case p.at(lexer.TokStruct):
			structTok := p.tok
			p.next() // consume 'struct'
			sd, err := p.parseStructDeclAt(structTok)
			if err != nil {
				return nil, err
			}
			f.Decls = append(f.Decls, sd)

		case p.at(lexer.TokEnum):
			enumTok := p.tok
			p.next() // consume 'enum'
			ed, err := p.parseEnumDeclAt(enumTok)
			if err != nil {
				return nil, err
			}
			f.Decls = append(f.Decls, ed)

		case p.at(lexer.TokLet):
			// Top-level const (non-pub)
			letTok := p.tok
			p.next() // consume 'let'
			cd, err := p.parseTopConstAt(letTok, false /*pub*/)
			if err != nil {
				return nil, err
			}
			f.Decls = append(f.Decls, cd)

		default:
			// Surface lexer errors immediately at top-level
			if p.at(lexer.TokErr) {
				t := p.tok
				return nil, ErrLexerError(t)
			}
			for !p.at(lexer.TokNewline) && !p.at(lexer.TokEOF) {
				p.next()
			}
			p.skipNewlines()
		}
	}
	return f, nil
}

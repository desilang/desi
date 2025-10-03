package check

import "strings"

// Reserved language words (superset of lexer keywords; includes 'none' and 'panic').
var reservedWords = map[string]struct{}{
	"package": {}, "import": {}, "from": {}, "as": {}, "pub": {},
	"def": {}, "let": {}, "mut": {}, "return": {},
	"if": {}, "elif": {}, "else": {}, "while": {}, "for": {}, "in": {},
	"match": {}, "struct": {}, "enum": {}, "type": {},
	"and": {}, "or": {}, "not": {}, "defer": {},
	"async": {}, "await": {},
	"true": {}, "false": {},
	"none": {}, "panic": {},
	// We also treat 'void' as reserved (banned identifier + banned type name).
	"void": {},
}

// Prelude/builtin names we don't allow users to shadow with bindings/defs.
var preludeBuiltins = map[string]struct{}{
	"print": {}, "len": {},
	// Core module roots and well-known std namespaces (avoid confusing shadowing).
	"io": {}, "fs": {}, "os": {}, "mem": {}, "str": {}, "math": {}, "fmt": {},
}

// isReservedIdent reports whether name is a reserved word (or 'void').
func isReservedIdent(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	_, ok := reservedWords[n]
	return ok
}

// isPreludeBuiltin reports whether name is a builtin/prelude symbol we forbid shadowing.
func isPreludeBuiltin(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	_, ok := preludeBuiltins[n]
	return ok
}
